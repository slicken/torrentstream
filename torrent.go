package main

import (
	"errors"
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
	// read file hash
	tr := t.Largest().NewReader()
	defer tr.Close()
	hash, err := readHash(tr, 64)
	if err != nil {
		return err
	}

	// download sub function
	subs := subDBdl(hash, lang)
	if len(subs) == 0 {
		return errors.New("no subtitles found")
	}

	t.Subs = subs
	return nil
}

// // FindSubInTorrent                   					-- not used
// func (t *T) FindSubInTorrent() []Subtitle {
// 	var subs []Subtitle

// 	var wg sync.WaitGroup
// 	for _, f := range t.Torrent.Files() {
// 		ext := filepath.Ext(f.Path())

// 		// look only for .vtt and .srt files
// 		if ext == ".vtt" || ext == ".srt" {

// 			wg.Add(1)
// 			go func(t *torrent.File) {
// 				defer wg.Done()

// 				log.Println("torrent contains subtitle", f.Path(), "downloading..")

// 				t.Download()

// 				file := filepath.Join(conf.FileDir, f.Path())
// 				if ext == ".srt" {
// 					var err error
// 					if file, err = subFileConvert(file); err != nil {
// 						return
// 					}
// 				}

// 				sub := Subtitle{
// 					Format: "vtt",
// 					Path:   file,
// 					Lang:   "Unknown",
// 				}
// 				subs = append(subs, sub)
// 			}(f)
// 		}
// 	}

// 	wg.Wait()
// 	return subs
// }
