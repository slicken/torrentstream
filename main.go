package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	conf     *Config
	search   *TorrentSites
	app      *App
	OMDB_KEY = os.Getenv("OMDB_KEY")
)

// App Config
type Config struct {
	Http     string
	FileDir  string
	Streams  int
	Seeders  int
	Sites    int
	Nodes    int
	ULRate   int
	DLRate   int
	Seed     bool
	Trackers bool
}

func main() {
	// HandleInterrupt in a goroutine
	go HandleInterrupt()

	var err error
	// configuration flags
	conf = new(Config)
	flag.StringVar(&conf.Http, "http", ":8080", "http server address")
	flag.StringVar(&conf.FileDir, "dir", "tmp", "directory for temporary files")
	flag.IntVar(&conf.Seeders, "seeders", 1, "minimum seeders to show torrent as result")
	flag.IntVar(&conf.Streams, "maximum", 0, "maximum active torrents (0=unlimited)")
	flag.IntVar(&conf.Sites, "site", 60, "check torrent sites (minutes)")
	flag.IntVar(&conf.Nodes, "nodes", 100, "maximum connections per torrent")
	flag.IntVar(&conf.ULRate, "ul", -1, "max bytes per second (upload)")
	flag.IntVar(&conf.DLRate, "dl", -1, "max bytes per second (download)")
	flag.BoolVar(&conf.Seed, "seed", false, "seed after download")
	flag.BoolVar(&conf.Trackers, "trackers", false, "add trackers to magnet links")
	flag.Parse()

	if OMDB_KEY == "" {
		log.Println("!!WARNING!! env key 'OMDB_KEY' is not set. Will not plot posters and movie info")
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

	// Initialize search
	search = InitializeSearch()
	search.Handler(conf.Sites)
	log.Printf("initialized torrent sites for %s, %s, %s, %s\n", tpb.Name, kat.Name, yts.Name, leetx.Name)

	// cashe movie list
	time.Sleep(time.Second)
	go InitializeMovieList()

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
	// Create a custom server with settings optimized for streaming
	server := &http.Server{
		Addr:    conf.Http,
		Handler: nil, // Use default handler
		// Increase timeouts to prevent interruptions during streaming
		ReadTimeout:       0, // No read timeout for streaming
		ReadHeaderTimeout: 30 * time.Second,
		WriteTimeout:      0, // No write timeout for streaming
		IdleTimeout:       0, // No idle timeout - keep connections alive
		// Configure for streaming with potentially imperfect data
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	log.Println("http server running at", conf.Http)
	log.Fatal(server.ListenAndServe())
}

// HandleInterrupt ..
func HandleInterrupt() {
	closeChan := make(chan os.Signal, 1)
	signal.Notify(closeChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	<-closeChan
	log.Printf("\nShutdown signal received. Cleaning up...\n")

	// Stop the app's goroutines and clean up torrents
	app.Shutdown()
	log.Println("Cleanup completed")

	os.Exit(0)
}
