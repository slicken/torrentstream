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

// newTorrent ..
func (app *App) newTorrent(request *http.Request, magnet metainfo.Magnet) (*T, error) {
	torrent, err := app.client.AddMagnet(magnet.String())
	if err != nil {
		return nil, err
	}
	// not important yet
	torrent.SetMaxEstablishedConns(conf.Nodes)

	select {
	case <-torrent.GotInfo():
		// continue...
	case <-request.Context().Done():
		torrent.Drop()
		return nil, errors.New("request Context aborted")
	case <-time.After(time.Minute):
		torrent.Drop()
		torrent.Closed()
		return nil, errors.New("torrent timeout")
	}

	return &T{
		Torrent:  torrent,
		ID:       magnet.InfoHash.String(),
		Activity: time.Now(),
	}, nil
}

// Close ..
func (t *T) close() {
	t.Drop()
	<-t.Closed()

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
func (t *T) LargestFile() *torrent.File {
	files := t.Files()
	sort.Slice(files, func(i, j int) bool { return files[i].Length() > files[j].Length() })
	return files[0]
}

// addSubtitles ..
func (t *T) addSubtitles(lang []string) error {
	t.Subs = append(t.Subs, t.findSubtitles()...)

	// if len(t.Subs) == 0 {
	// 	tr := t.LargestFile().NewReader()
	// 	defer tr.Close()
	// 	hash, err := readHash(tr, 64)
	// 	if err != nil {
	// 		return fmt.Errorf("hashing: %s", err)
	// 	} else {
	// 		t.Subs = append(t.Subs, subDBdl(hash, lang)...)
	// 	}
	// }

	return nil
}

// FindSubInTorrent
func (t *T) findSubtitles() []Subtitle {
	var subs []Subtitle

	for _, tf := range t.Torrent.Files() {
		ext := filepath.Ext(tf.Path())

		if ext == ".vtt" || ext == ".srt" {

			tf.Download()
			file := filepath.Join(conf.FileDir, tf.Path())

			for {
				fileInfo, err := os.Stat(file)
				if err == nil && fileInfo.Size() > 0 {
					break
				}

				time.Sleep(100 * time.Microsecond)
			}

			if ext == ".srt" {
				var err error
				if file, err = subFileConvert(file); err != nil {
					fmt.Println(err)
					continue
				}
			}

			var sub Subtitle
			sub.Format = "vtt"
			sub.Path = file
			sub.Lang = detectLanguageFromFilename(file)

			subs = append(subs, sub)
			log.Printf("[%s] subitle found @ %s\n", sub.Lang, filepath.Base(file))
		}
	}

	return subs
}

// TrackConnectionActivity ..
func (t *T) TrackConnectionActivity(r *http.Request) {
	t.Lock()
	t.Conn++
	t.Activity = time.Now()
	t.Unlock()

	go func() {
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
