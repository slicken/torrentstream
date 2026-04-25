package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
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
	FFmpeg   bool
	// FFmpegPath is resolved with exec.LookPath when FFmpeg is true.
	FFmpegPath string
}

// Usage prints a concise, grouped help screen.
func Usage() {
	exe := filepath.Base(os.Args[0])
	if exe == "" {
		exe = "torrentstream"
	}

	fmt.Fprintf(flag.CommandLine.Output(), `torrentstream

Private torrent-to-browser streaming service.

Usage:
  %s [options]

Server:
  -http <addr>          HTTP listen address                  default: :8080
  -dir <path>           temporary download directory          default: tmp
  -maximum <n>          maximum active torrents               default: 0 (unlimited)

Torrent:
  -seeders <n>          minimum seeders in search results     default: 1
  -nodes <n>            maximum peer connections per torrent  default: 100
  -trackers             enable tracker peer discovery         default: true
  -seed                 keep seeding after download           default: false

Bandwidth:
  -dl <bytes/sec>       download rate limit                   default: -1 (unlimited)
  -ul <bytes/sec>       upload rate limit                     default: -1 (unlimited)

Search:
  -site <minutes>       torrent-site health check interval    default: 60

Playback:
  -ffmpeg               transcode to browser-friendly MP4     default: false
  -ffmpeg.path <path>   FFmpeg binary path                    default: ffmpeg

Examples:
  %s -http :8080
  %s -ffmpeg -maximum 5

`, exe, exe, exe)
}

func main() {
	// HandleInterrupt in a goroutine
	go HandleInterrupt()

	var err error
	// configuration flags
	conf = new(Config)
	flag.Usage = Usage
	flag.StringVar(&conf.Http, "http", ":8080", "http server address")
	flag.StringVar(&conf.FileDir, "dir", "tmp", "directory for temporary files")
	flag.IntVar(&conf.Seeders, "seeders", 1, "minimum seeders to show torrent as result")
	flag.IntVar(&conf.Streams, "maximum", 0, "maximum active torrents (0=unlimited)")
	flag.IntVar(&conf.Sites, "site", 60, "check torrent sites (minutes)")
	flag.IntVar(&conf.Nodes, "nodes", 100, "maximum connections per torrent")
	flag.IntVar(&conf.ULRate, "ul", -1, "max bytes per second (upload)")
	flag.IntVar(&conf.DLRate, "dl", -1, "max bytes per second (download)")
	flag.BoolVar(&conf.Seed, "seed", false, "seed after download")
	flag.BoolVar(&conf.Trackers, "trackers", true, "enable trackers for peer discovery")
	flag.BoolVar(&conf.FFmpeg, "ffmpeg", false, "transcode via FFmpeg for browser-friendly H.264/AAC (serves /ffmpeg instead of raw /stream)")
	flag.StringVar(&conf.FFmpegPath, "ffmpeg.path", "ffmpeg", "path to ffmpeg binary (used when -ffmpeg is set)")
	flag.Parse()

	if conf.FFmpeg {
		p, err := exec.LookPath(conf.FFmpegPath)
		if err != nil {
			log.Fatal("-ffmpeg: ffmpeg binary not found:", err)
		}
		conf.FFmpegPath = p
		log.Println("FFmpeg transcoding enabled:", p)
	}

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
	http.HandleFunc("/touch", touch)
	http.HandleFunc("/stream", stream)
	http.HandleFunc("/ffmpeg", ffmpegStream)
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

	KillAllFFmpegProcesses()

	// Stop the app's goroutines and clean up torrents
	app.Shutdown()
	log.Println("Cleanup completed")

	os.Exit(0)
}
