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

// Ts is main program struct
type Ts struct {
	client *torrent.Client
	m      map[string]*T
	sync.RWMutex
}

// NewTs initalizes new toorent client
func NewTs() (*Ts, error) {
	cli, err := NewTorrentClient()
	if err != nil {
		return nil, err
	}

	ts := &Ts{
		client: cli,
		m:      make(map[string]*T),
		// RWMutex: sync.RWMutex{},
	}
	// maintain and flush un-used content
	go ts.Handler()

	return ts, nil
}

// Load adds or returns existing torrent
func (ts *Ts) Load(r *http.Request, m metainfo.Magnet) (*T, error) {
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
	log.Println("adding", id)
	t, err := ts.NewTorrent(r, m)
	if err != nil {
		return nil, err
	}

	// add subtitles if possible - run subfunctions
	if err = t.AddSubtitles([]string{"en", "es", "se"}); err != nil {
		log.Printf("could not add subtitles: %v\n", err)
	}

	// add to history
	hist = append(hist, History{time.Now(), t.Name()})

	// add to map and
	log.Println("streaming", id)
	ts.Lock()
	ts.m[id] = t
	ts.Unlock()
	return t, nil
}

// Get torrent
func (ts *Ts) Get(id string) (*T, bool) {
	ts.RLock()
	defer ts.RUnlock()

	t, ok := ts.m[id]
	if !ok {
		return nil, false
	}
	return t, true
}

// Delete torrent
func (ts *Ts) Delete(id string) bool {
	ts.Lock()
	defer ts.Unlock()

	if _, ok := ts.m[id]; !ok {
		return false
	}
	delete(ts.m, id)
	return true
}

// Handler cleans up stopped connections
func (ts *Ts) Handler() {
	for {
		ts.RLock()
		m := ts.m
		ts.RUnlock()

		for k, t := range m {
			t.RLock()
			since := time.Since(t.Activity)
			conn := t.Conn
			t.RUnlock()

			if 0 >= conn && since > (conf.Idle*time.Second) {
				if t, ok := ts.Get(k); ok {
					t.Close()
				}
				ts.Delete(k)
			}
		}
		time.Sleep(10 * time.Second)
	}
}
