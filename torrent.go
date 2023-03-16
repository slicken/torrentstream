package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"golang.org/x/time/rate"
)

// T is app torrents
type T struct {
	*torrent.Torrent
	Subs     []Subtitle
	ID       string
	Conn     int
	Activity time.Time
	sync.RWMutex
}

// NewTorrentClient ..
func NewTorrentClient() (*torrent.Client, error) {
	cfg := torrent.NewDefaultClientConfig()
	//cfg.Seed = conf.Seed
	// cfg.NoUpload = true
	cfg.DisableTrackers = true
	if conf.FileDir != "" {
		cfg.DataDir = conf.FileDir
	}
	if conf.DownloadRate != -1 {
		cfg.DownloadRateLimiter = rate.NewLimiter(rate.Limit(conf.DownloadRate), 1<<20)
	}
	if conf.UploadRate != -1 {
		cfg.UploadRateLimiter = rate.NewLimiter(rate.Limit(conf.UploadRate), 256<<10)
	}
	return torrent.NewClient(cfg)
}

// NewTorrent ..
func (ts *Ts) NewTorrent(r *http.Request, m metainfo.Magnet) (*T, error) {
	t, err := ts.client.AddMagnet(m.String())
	if err != nil {
		return nil, err
	}
	// not important yet
	t.SetMaxEstablishedConns(conf.Node)

	select {
	case <-t.GotInfo():
		// continue...
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
		ID:       m.InfoHash.String(),
		Activity: time.Now(),
	}, nil
}

// Close ..
func (t *T) Close() {
	t.Drop()
	<-t.Closed()
	time.Sleep(time.Second)

	// remove subtitles
	for _, file := range t.Subs {
		path := filepath.Join(conf.FileDir, file.Path)
		path, _ = filepath.Abs(path)
		if err := os.RemoveAll(path); err != nil {
			log.Printf("error delete file %q: %v\n", path, err)
		}
	}
	// remove files
	for _, file := range t.Files() {
		path := filepath.Join(conf.FileDir, file.Path())
		path, _ = filepath.Abs(path)
		if err := os.RemoveAll(path); err != nil {
			log.Printf("error delete file %q: %v\n", path, err)
		}
	}
	// remove torrent dir
	dir, _ := filepath.Split(t.Files()[0].Path())
	if dir != "" && dir != conf.FileDir {
		path := filepath.Join(conf.FileDir, dir)
		path, _ = filepath.Abs(path)
		if err := os.RemoveAll(path); err != nil {
			log.Printf("error delete directory %q: %v\n", dir, err)
		}
	}

	log.Println("closed", t.Name())
}

// Largest ..
func (t *T) Largest() *torrent.File {
	files := t.Files()
	sort.Slice(files, func(i, j int) bool { return files[i].Length() > files[j].Length() })
	return files[0]
}

func (t *T) activityCtx(r *http.Request) {
	go func() {
		t.Lock()
		t.Conn++
		t.Activity = time.Now()
		t.Unlock()

		for {
			select {
			case <-r.Context().Done():
				t.Lock()
				t.Conn--
				t.Unlock()
				return

			default:
				t.Lock()
				t.Activity = time.Now()
				t.Unlock()
				time.Sleep(time.Second)
			}
		}
	}()
}

// AddSubtitles ..
func (t *T) AddSubtitles(lang []string) error {
	// tr := t.Largest().NewReader()
	// defer tr.Close()
	// hash, err := readHash(tr, 64)
	// if err != nil {
	// 	return fmt.Errorf("hashing: %s", err)
	// } else {
	// t.Subs = append(t.Subs, subDBdl(hash, lang)...)
	// }
	// // download sub function
	t.Subs = append(t.Subs, t.FindSubInTorrent()...)

	if len(t.Subs) == 0 {
		return errors.New("not subtitles found")
	}

	return nil
}

// FindSubInTorrent
func (t *T) FindSubInTorrent() []Subtitle {
	var subs []Subtitle

	for _, f := range t.Torrent.Files() {
		ext := filepath.Ext(f.Path())

		if ext == ".vtt" || ext == ".srt" {

			f.Download()
			file := filepath.Join(conf.FileDir, f.Path())

			for {
				fi, err := os.Stat(file)
				if err != nil {
					goto SLEEP
				}
				if fi.Size() > 0 {
					break
				}
			SLEEP:
				time.Sleep(200 * time.Microsecond)
			}

			if ext == ".srt" {
				var err error
				if file, err = subFileConvert(file); err != nil {
					fmt.Println(err)
					continue
				}
			}

			// file = file[len(conf.FileDir)+1:]

			var sub Subtitle
			sub.Format = "vtt"
			sub.Path = file
			sub.Lang = "en"

			subs = append(subs, sub)
			log.Println("subtitle @", file)
		}
	}

	return subs
}
