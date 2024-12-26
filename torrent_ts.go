package main

import (
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
)

// TorrentStream is main program struct
type TorrentStream struct {
	client *torrent.Client
	m      map[string]*T
	sync.RWMutex
}

// NewTorrentStream initalizes new toorent client
func StartApplication() (*TorrentStream, error) {
	cli, err := NewTorrentClient()
	if err != nil {
		return nil, err
	}

	ts := &TorrentStream{
		client:  cli,
		m:       make(map[string]*T),
		RWMutex: sync.RWMutex{},
	}

	go ts.Handler()

	return ts, nil
}

// NewTorrent ..
func (ts *TorrentStream) NewTorrent(r *http.Request, m metainfo.Magnet) (*T, error) {
	t, err := ts.client.AddMagnet(m.String())
	if err != nil {
		return nil, err
	}

	// t.SetMaxEstablishedConns(conf.Nodes) // not important yet

	select {
	case <-t.GotInfo():
		// great, continue..
	case <-r.Context().Done():
		t.Drop()
		return nil, errors.New("request ctx abort")
	case <-time.After(time.Minute):
		t.Drop()
		t.Closed()
		return nil, errors.New("torrent timeout")
	}

	return &T{
		Torrent:  t,
		Activity: time.Now(),
	}, nil
}

// Load adds or returns existing torrent
func (ts *TorrentStream) Load(r *http.Request, m metainfo.Magnet) (*T, error) {
	if len(ts.m) > conf.Streams {
		return nil, errors.New("too many streams")
	}
	id := m.InfoHash.String()

	// check active map
	ts.RLock()
	t, ok := ts.m[id]
	ts.RUnlock()
	if ok {
		return t, nil
	}

	// add torrent
	log.Println("adding", m.DisplayName)
	t, err := ts.NewTorrent(r, m)
	if err != nil {
		return nil, err
	}

	// add subtitles if possible - run subfunctions
	if err = t.AddSubtitles([]string{"en", "eng", "se"}); err != nil {
		log.Println(err)
	}

	// add to history
	hist = append(hist, History{time.Now(), t.Name()})

	// add to map and
	ts.Lock()
	ts.m[id] = t
	ts.Unlock()
	log.Println("streaming", t.Name())

	return t, nil
}

// Get torrent
func (ts *TorrentStream) Get(id string) (*T, bool) {
	ts.RLock()
	defer ts.RUnlock()

	t, ok := ts.m[id]
	if !ok {
		return nil, false
	}
	return t, true
}

// Delete torrent
func (ts *TorrentStream) Delete(id string) bool {
	ts.Lock()
	defer ts.Unlock()

	ts.m[id].Close()

	if _, ok := ts.m[id]; !ok {
		return false
	}

	delete(ts.m, id)
	return true
}

func (ts *TorrentStream) Handler() {
	for {
		ts.RLock()
		m := ts.m
		ts.RUnlock()

		now := time.Now()

		for id, t := range m {
			t.RLock()
			since := time.Since(t.Activity)
			conn := t.Conn
			t.RUnlock()

			if conn == 0 && since > conf.Idle {
				t.Lock()
				if now.Sub(t.Activity) > conf.Idle {
					t.Unlock()
					ts.Delete(id)
				} else {
					t.Unlock()
				}
			}
		}
		time.Sleep(10 * time.Second)
	}
}
