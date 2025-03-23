package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// globals

var (
	conf     *Config
	search   *TorrentSites
	app      *App
	OMDB_KEY = os.Getenv("OMDB_KEY")
)

// Config global
type Config struct {
	Http         string
	FileDir      string
	Idle         time.Duration
	Streams      int
	Seeders      int
	Sites        int
	Nodes        int
	UploadRate   int
	DownloadRate int
	Seed         bool
	Trackers     bool
}

func main() {
	var err error
	// configuration flags
	var tmpIdle string
	conf = new(Config)
	flag.StringVar(&conf.Http, "http", ":8080", "http server address")
	flag.StringVar(&conf.FileDir, "dir", "tmp", "directory for temporary files")
	// flag.DurationVar(&conf.Idle, "idle", 15*time.Minute, "idle time before closing")
	flag.StringVar(&tmpIdle, "idle", "15m", "idle time before closing")
	flag.IntVar(&conf.Seeders, "seeders", 1, "minimum seeders")
	flag.IntVar(&conf.Streams, "maximum", 50, "maximum active torrents")
	// less important
	flag.IntVar(&conf.Sites, "site", 100, "check torrent sites (minutes)")
	flag.IntVar(&conf.Nodes, "nodes", 100, "maximum connections per torrent")
	flag.IntVar(&conf.UploadRate, "ul", -1, "max bytes per second (upload)")
	flag.IntVar(&conf.DownloadRate, "dl", -1, "max bytes per second (download)")
	flag.BoolVar(&conf.Seed, "seed", false, "seed after download")
	flag.BoolVar(&conf.Trackers, "trackers", false, "add trackers to magnet links")
	flag.Parse()

	if OMDB_KEY == "" {
		log.Println("!!WARNING!! env key 'OMDB' is not set. Will not plot posters and movie info")
	}

	// Parse the duration and assign it to conf.Idle
	conf.Idle, err = time.ParseDuration(tmpIdle)
	if err != nil {
		log.Fatalln("could not parse Idle time:", err)
	}

	// log to file with '--log' arg
	if strings.Contains(fmt.Sprint(os.Args), "--log") {
		logName := time.Now().Format("01021504") + ".log"
		logFile, err := os.Create(logName)
		if err != nil {
			log.Fatalf("could not create logfile %q: %v", logFile.Name(), err)
		}
		log.SetOutput(io.MultiWriter(os.Stderr, logFile))
		log.Printf("successfully created logfile %q.\n", logFile.Name())
	}

	if _, err := os.Stat(conf.FileDir); os.IsNotExist(err) {
		os.MkdirAll(conf.FileDir, 0755)
	}
	log.Printf("temporary files stores in %q", conf.FileDir)

	// Ts is main torrent diver program
	app, err = New()
	if err != nil {
		log.Fatal("could not initalize torrent client:", err)
	}

	// meta-search on external torrent sites
	tpb.ScrapeFunc = tpbSearch
	kat.ScrapeFunc = katSearch
	search = &TorrentSites{
		sites: []*TorrentSite{
			tpb,
			kat,
		},
	}

	search.Handler(conf.Sites)
	log.Printf("initalized torrent sites for %s, %s\n", tpb.Name, kat.Name)

	subDB.search = subDBSearch
	subDB.download = subDBDownload

	// http handlers
	http.Handle("/favicon.ico", http.NotFoundHandler())
	http.Handle("/www/", http.StripPrefix("/www/", http.FileServer(http.Dir("www"))))
	http.Handle("/tmp/", http.StripPrefix("/tmp/", http.FileServer(http.Dir(conf.FileDir))))
	http.HandleFunc("/", index)
	http.HandleFunc("/play", play)
	http.HandleFunc("/stream", stream)
	http.HandleFunc("/stats", stats)

	// https server
	go func() {
		log.Println("http server running at", conf.Http)
		log.Fatal(http.ListenAndServe(conf.Http, nil))
	}()

	HandleInterrupt()
}

// HandleInterrupt ..
func HandleInterrupt() {
	closeChan := make(chan os.Signal, 1)
	signal.Notify(closeChan, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	log.Printf("\n%+v received. shutting down.\n", <-closeChan)

	// Create a channel to signal when cleanup is complete
	cleanupDone := make(chan struct{})
	go func() {
		// Stop the app's goroutines and clean up torrents
		app.Close()

		// Clean up files
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

		close(cleanupDone)
	}()

	// Wait for cleanup with timeout
	select {
	case <-cleanupDone:
		log.Println("Cleanup completed successfully")
	case <-time.After(3 * time.Second):
		log.Println("Cleanup timed out, forcing exit")
		// Force kill any remaining processes
		if p, err := os.FindProcess(os.Getpid()); err == nil {
			p.Kill()
		}
	}

	os.Exit(0)
}
