package main

import (
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"golang.org/x/time/rate"
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
	cfg.DisableTrackers = true

	if conf.FileDir != "" {
		cfg.DataDir = conf.FileDir
	}

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

	// More lenient peer settings
	cfg.DisableIPv6 = true       // Disable IPv6 to reduce connection issues
	cfg.DisableUTP = true        // Disable UTP to use only TCP
	cfg.DisableWebtorrent = true // Disable WebTorrent to avoid browser-specific issues

	return torrent.NewClient(cfg)
}

// Add will add or returns existing torrent
func (app *App) Add(request *http.Request, magnet metainfo.Magnet) (*T, error) {
	if conf.Streams > 0 && len(app.torrents) > conf.Streams {
		return nil, errors.New("maximum number of active streams reached. Please wait for some streams to finish.")
	}
	id := magnet.InfoHash.String()

	// check active map
	app.RLock()
	t, ok := app.torrents[id]
	app.RUnlock()
	if ok {
		return t, nil
	}

	// add torrent
	log.Println("adding", magnet.DisplayName)
	t, err := app.newTorrent(request, magnet)
	if err != nil {
		return nil, err
	}

	// add subtitles if possible - run subfunctions
	if err = t.addSubtitles([]string{"en", "se"}); err != nil {
		log.Println(err)
	}

	// add to history
	history = append(history, History{time.Now(), t.Name()})

	// add to map
	log.Println("streaming", t.Name())
	app.Lock()
	app.torrents[id] = t
	app.Unlock()
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

// Close stops all goroutines and cleans up resources
func (app *App) Close() {
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

	// Clean up all torrents first
	app.Lock()
	torrents := make([]string, 0, len(app.torrents))
	for id := range app.torrents {
		torrents = append(torrents, id)
	}
	app.Unlock()

	// Delete torrents outside the lock
	for _, id := range torrents {
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
}

// cleans up non-streaming torrents
func (app *App) handler() {
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-app.stopChan:
				return
			case <-ticker.C:
				var torrentsToDelete []string

				app.RLock()
				for id, torrent := range app.torrents {
					torrent.RLock()
					conn := torrent.Conn
					torrent.RUnlock()

					if conn == 0 {
						torrentsToDelete = append(torrentsToDelete, id)
					}
				}
				app.RUnlock()

				// Delete torrents outside the read lock to avoid deadlocks
				for _, id := range torrentsToDelete {
					app.Delete(id)
				}
			}
		}
	}()
}
