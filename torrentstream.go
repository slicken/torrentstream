package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"golang.org/x/time/rate"
)

const (
	torrentCleanupInterval = 10 * time.Second
	torrentIdleTimeout     = 5 * time.Minute
)

// App is main program struct
type App struct {
	client   *torrent.Client
	torrents map[string]*T
	stopChan chan struct{}
	wg       sync.WaitGroup
	sync.RWMutex
}

// New initalizes main application
func New() (*App, error) {
	cli, err := NewClient()
	if err != nil {
		return nil, err
	}

	app := &App{
		client:   cli,
		torrents: make(map[string]*T),
		stopChan: make(chan struct{}),
	}

	// maintain and flush un-used content
	app.handler()

	return app, nil
}

// NewClient ..
func NewClient() (*torrent.Client, error) {
	cfg := torrent.NewDefaultClientConfig()

	cfg.Seed = conf.Seed
	cfg.DisableTrackers = !conf.Trackers

	if conf.FileDir != "" {
		cfg.DataDir = conf.FileDir
	}

	// Enable more connection options for better streaming
	cfg.DisableIPv6 = false       // Enable IPv6 for more peers
	cfg.DisableUTP = false        // Enable UTP for better connections
	cfg.DisableWebtorrent = false // Enable WebTorrent for browser peers
	cfg.DisablePEX = false        // Enable peer exchange
	cfg.NoDHT = false             // Enable DHT for more peers

	// Configure download rate limiter
	if conf.DLRate > 0 {
		// Use a burst size of 1MB for downloads
		burst := 1 << 20 // 1MB burst
		if conf.DLRate < burst {
			burst = conf.DLRate
		}
		cfg.DownloadRateLimiter = rate.NewLimiter(rate.Limit(conf.DLRate), burst)
		log.Printf("Download rate limited to %d bytes/s (burst: %d bytes)", conf.DLRate, burst)
	}

	// Configure upload rate limiter
	if conf.ULRate > 0 {
		// Use a burst size of 256KB for uploads
		burst := 256 << 10 // 256KB burst
		if conf.ULRate < burst {
			burst = conf.ULRate
		}
		cfg.UploadRateLimiter = rate.NewLimiter(rate.Limit(conf.ULRate), burst)
		log.Printf("Upload rate limited to %d bytes/s (burst: %d bytes)", conf.ULRate, burst)
	}

	return torrent.NewClient(cfg)
}

// List all loaded storrens
func (app *App) List() []string {
	app.RLock()
	defer app.RUnlock()

	var torrents []string
	for _, t := range app.torrents {
		torrents = append(torrents, t.Name())
	}

	return torrents
}

// ListIDs returns the info hashes for all loaded torrents.
func (app *App) ListIDs() []string {
	app.RLock()
	defer app.RUnlock()

	ids := make([]string, 0, len(app.torrents))
	for id := range app.torrents {
		ids = append(ids, id)
	}

	return ids
}

// Add will add or returns existing torrent
func (app *App) Add(request *http.Request, magnet metainfo.Magnet) (*T, error) {
	id := magnet.InfoHash.String()

	// check active map
	app.RLock()
	t, ok := app.torrents[id]
	active := len(app.torrents)
	app.RUnlock()
	if ok {
		t.Touch()
		return t, nil
	}
	if conf.Streams > 0 && active >= conf.Streams {
		return nil, errors.New("maximum number of active streams reached. Please wait for some streams to finish.")
	}

	// add torrent
	log.Println("adding", magnet.DisplayName)
	t, err := app.newTorrent(request, magnet)
	if err != nil {
		return nil, err
	}

	// add to map
	log.Println("streaming", t.Name())
	app.Lock()
	if existing, ok := app.torrents[id]; ok {
		app.Unlock()
		existing.Touch()
		return existing, nil
	}
	if conf.Streams > 0 && len(app.torrents) >= conf.Streams {
		app.Unlock()
		t.close()
		return nil, errors.New("maximum number of active streams reached. Please wait for some streams to finish.")
	}
	app.torrents[id] = t
	app.Unlock()

	// add to history after the torrent is active
	history = append([]History{{time.Now(), t.Name()}}, history...)

	go func() {
		if err := t.addSubtitles([]string{"en", "se"}); err != nil {
			log.Println(err)
		}
	}()

	return t, nil
}

// Get torrent
func (app *App) Get(id string) (*T, bool) {
	app.RLock()
	defer app.RUnlock()

	t, ok := app.torrents[id]
	if !ok {
		return nil, false
	}
	return t, true
}

// Delete torrent
func (app *App) Delete(id string) bool {
	app.Lock()
	torrent, exist := app.torrents[id]
	if !exist {
		app.Unlock()
		return false
	}

	delete(app.torrents, id)
	app.Unlock()

	if torrent != nil {
		torrent.close()
	}
	return true
}

func (app *App) DeleteIfIdle(id string, idleFor time.Duration) bool {
	app.Lock()
	torrent, exist := app.torrents[id]
	if !exist {
		app.Unlock()
		return false
	}
	if !torrent.IdleFor(idleFor) {
		app.Unlock()
		return false
	}

	delete(app.torrents, id)
	app.Unlock()

	torrent.close()
	return true
}

// Shutdown stops all goroutines and cleans up resources
func (app *App) Shutdown() {
	// Signal all goroutines to stop
	close(app.stopChan)

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		app.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Goroutines finished successfully
	case <-time.After(2 * time.Second):
		log.Println("Warning: goroutines did not finish in time")
	}

	// Get list of all torrents by map key and delete them.
	for _, id := range app.ListIDs() {
		app.Delete(id)
	}

	// Close the torrent client and wait for it to finish
	if app.client != nil {
		app.client.Close()
		// Wait for the client to fully close with timeout
		select {
		case <-app.client.Closed():
			// Client closed successfully
		case <-time.After(2 * time.Second):
			log.Println("Warning: torrent client did not close in time")
		}
	}

	// Clean up the temporary directory
	wd, _ := os.Getwd()
	if conf.FileDir != "" && conf.FileDir != wd {
		// Try to remove the directory multiple times with increasing delays
		for i := 0; i < 3; i++ {
			if err := os.RemoveAll(conf.FileDir); err != nil {
				log.Printf("attempt %d: error deleting directory %q: %v\n", i+1, conf.FileDir, err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			break
		}
	}

	// Clean up database file
	dbFile := filepath.Join(conf.FileDir, ".torrent.bolt.db")
	if err := os.RemoveAll(dbFile); err != nil {
		log.Printf("error deleting file %q: %v\n", dbFile, err)
	}
}

// cleans up non-streaming torrents
func (app *App) handler() {
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		ticker := time.NewTicker(torrentCleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-app.stopChan:
				return
			case <-ticker.C:
				var torrentsToDelete []string

				app.RLock()
				for id, torrent := range app.torrents {
					if torrent.IdleFor(torrentIdleTimeout) {
						torrentsToDelete = append(torrentsToDelete, id)
					}
				}
				app.RUnlock()

				// Delete torrents outside the read lock, re-checking idleness before dropping.
				for _, id := range torrentsToDelete {
					app.DeleteIfIdle(id, torrentIdleTimeout)
				}
			}
		}
	}()
}
