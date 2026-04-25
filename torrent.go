package main

import (
	"context"
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

// Adds a new torrent
func (app *App) newTorrent(request *http.Request, magnet metainfo.Magnet) (*T, error) {
	torrent, err := app.client.AddMagnet(magnet.String())
	if err != nil {
		return nil, err
	}
	torrent.SetMaxEstablishedConns(conf.Nodes)

	// Create a longer timeout context
	ctx, cancel := context.WithTimeout(request.Context(), 3*time.Minute)
	defer cancel()

	select {
	case <-torrent.GotInfo():
		// Success case
		return &T{
			Torrent:  torrent,
			ID:       magnet.InfoHash.String(),
			Activity: time.Now(),
		}, nil
	case <-ctx.Done():
		torrent.Drop()
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("timeout waiting for torrent info after 3 minutes")
		}
		return nil, fmt.Errorf("request cancelled: %v", ctx.Err())
	}
}

// Close ..
func (t *T) close() {
	t.Drop()
	<-t.Closed()

	// remove subtitles
	for _, file := range t.Subtitles() {
		path := filepath.Join(conf.FileDir, file.Path)
		path, _ = filepath.Abs(path)
		if err := os.RemoveAll(path); err != nil {
			log.Printf("error delete file %q: %v\n", path, err)
		}
	}
	// remove files
	files := t.Files()
	for _, file := range files {
		path := filepath.Join(conf.FileDir, file.Path())
		path, _ = filepath.Abs(path)
		if err := os.RemoveAll(path); err != nil {
			log.Printf("error delete file %q: %v\n", path, err)
		}
	}
	// remove torrent dir
	if len(files) > 0 {
		dir, _ := filepath.Split(files[0].Path())
		if dir != "" && dir != conf.FileDir {
			path := filepath.Join(conf.FileDir, dir)
			path, _ = filepath.Abs(path)
			if err := os.RemoveAll(path); err != nil {
				log.Printf("error delete directory %q: %v\n", dir, err)
			}
		}
	}

	log.Println("closed", t.Name())
}

// Largest ..
func (t *T) LargestFile() *torrent.File {
	files := t.Files()
	if len(files) == 0 {
		return nil
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Length() > files[j].Length() })
	return files[0]
}

// addSubtitles ..
func (t *T) addSubtitles(lang []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	subs := t.findSubtitles(ctx)
	if len(subs) == 0 {
		return nil
	}

	t.Lock()
	t.Subs = append(t.Subs, subs...)
	t.Unlock()

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

func (t *T) Subtitles() []Subtitle {
	t.RLock()
	defer t.RUnlock()

	subs := make([]Subtitle, len(t.Subs))
	copy(subs, t.Subs)
	return subs
}

// FindSubInTorrent
func (t *T) findSubtitles(ctx context.Context) []Subtitle {
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

				select {
				case <-ctx.Done():
					return subs
				case <-time.After(250 * time.Millisecond):
				}
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

func (t *T) Touch() {
	t.Lock()
	t.Activity = time.Now()
	t.Unlock()
}

func (t *T) IdleFor(d time.Duration) bool {
	t.RLock()
	defer t.RUnlock()

	return t.Conn == 0 && time.Since(t.Activity) >= d
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
				if t.Conn > 0 {
					t.Conn--
				}
				t.Activity = time.Now()
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
