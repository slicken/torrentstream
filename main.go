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
	conf   *Config
	search *TorrentSites
	ts     *Ts
)

// Config global
type Config struct {
	Http         string
	FileDir      string
	Idle         time.Duration
	Streams      int
	Seeders      int
	Sites        int
	Node         int
	UploadRate   int
	DownloadRate int
	Seed         bool
}

func main() {
	// configuration flags
	conf = new(Config)
	flag.StringVar(&conf.Http, "http", ":8080", "http server address")
	flag.StringVar(&conf.FileDir, "dir", "torrents", "torrents directory for temp downloads")
	flag.DurationVar(&conf.Idle, "idle", 60, "maximum torrent idle in seconds")
	flag.IntVar(&conf.Seeders, "seeders", 5, "minimum seeders for torrent to show as a result")
	flag.IntVar(&conf.Streams, "maximum", 50, "maximum active torrents")
	// less important
	flag.IntVar(&conf.Sites, "site", 20, "site handler check in minutes minutes")
	flag.IntVar(&conf.Node, "node", 100, "maximum node connections per torrent")
	flag.IntVar(&conf.UploadRate, "ul", -1, "max bytes per second for upload")
	flag.IntVar(&conf.DownloadRate, "dl", -1, "max bytes per second for download")
	flag.BoolVar(&conf.Seed, "seed", false, "seed after download is complete")
	flag.Parse()

	// check OMDB key
	if omdbKey == "" {
		log.Println("WARNING enviroment key OMDB is not set. Will not plot movie info and posters")
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

	// check if download dir is set
	if conf.FileDir != "" && !filepath.IsAbs(conf.FileDir) {
		conf.FileDir, _ = filepath.Abs(conf.FileDir)
		log.Printf("torrent directory is set %s\n", conf.FileDir)
	}

	// torrent and app main program
	var err error
	ts, err = NewTs()
	if err != nil {
		log.Fatal("could not initalize torrent client:", err)
	}

	// meta-search on external torrent sites
	tpb.f = tpbSearch
	kat.f = katSearch
	search = &TorrentSites{
		sites: []*TorrentSite{
			tpb,
			kat,
		},
	}
	go search.Handler(conf.Sites)
	log.Printf("torrent sites handler initalized for %s, %s\n", tpb.Name, kat.Name)

	// subtitle functions					--- not done!
	// subDB.s = subDBSearch
	subDB.d = subDBDownload

	// http handlers
	http.Handle("/www/", http.StripPrefix("/www", http.FileServer(http.Dir("www"))))
	http.Handle("/blob/", http.StripPrefix("/blob", http.FileServer(http.Dir(conf.FileDir))))
	http.Handle("/favicon.ico", http.NotFoundHandler())
	http.HandleFunc("/", index)
	http.HandleFunc("/play", play)
	http.HandleFunc("/stream", stream)
	http.HandleFunc("/stats", stats)

	// https server
	go func() {
		log.Println("http serving at", conf.Http)
		log.Fatal(http.ListenAndServe(conf.Http, nil))
	}()

	HandleInterrupt()
}

// HandleInterrupt ..
func HandleInterrupt() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case s := <-sig:
		fmt.Println()
		log.Printf("%+v recived. shutting down.\n", s)

		ts.Lock()
		for i := range ts.m {
			ts.m[i].Close()
			delete(ts.m, i)
		}
		ts.Unlock()
	}
	dbFile := filepath.Join(conf.FileDir, ".torrent.bolt.db")
	if err := os.RemoveAll(dbFile); err != nil {
		log.Printf("error delete file %q: %v\n", dbFile, err)
	}
	wd, _ := os.Getwd()
	if conf.FileDir != "" && conf.FileDir != wd {
		if err := os.RemoveAll(conf.FileDir); err != nil {
			log.Printf("error delete dir %q: %v\n", conf.FileDir, err)
		}
	}
	time.Sleep(time.Second)
	os.Exit(0)
}
