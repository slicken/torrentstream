package main

import (
	"html/template"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/anacrolix/torrent/metainfo"
)

const (
	// msgbox types
	msgNone     = iota
	msgWelcome  // 1
	msgResult   // 2
	msgNoresult // 3
)

var (
	tmplIndex = template.Must(template.ParseFiles("www/index.html"))
	tmplPlay  = template.Must(template.ParseFiles("www/play.html"))

	// stats page
	tmplStats = template.Must(template.New("stats").Parse(`
	Boot    {{.Time}}
	Visits  {{.Visits}}
	Streams {{.Streams}}
	Active  {{len .Ts}}
	
	--- HISTORY ---
	{{range .History}}
	{{.Time}}	{{.File}}
	{{end}}
	`))

	// stats page
	boot    = time.Now()
	visits  int
	streams int
	history []History
)

// History for stats
type History struct {
	Time time.Time
	File string
}

func stats(w http.ResponseWriter, r *http.Request) {
	// print app statistics
	var info = struct {
		Time    time.Time
		Visits  int
		Streams int
		Ts      map[string]*T
		History []History
	}{
		Time:    boot,
		Visits:  visits,
		Streams: streams,
		Ts:      app.torrents,
		History: history,
	}

	tmplStats.Execute(w, info)
}

func index(w http.ResponseWriter, r *http.Request) {
	visits++ // stats

	var data = struct {
		Msg      int
		Search   string
		Category string
		T        []*Torrent
	}{
		Msg: msgWelcome,
	}

	// http post?
	if r.Method == http.MethodPost {
		data.Search = r.FormValue("search")
		data.Category = r.FormValue("category")
		// make search and collect results
		data.T = search.SearchTorrent(data.Search, data.Category)

		// check if we got results
		data.Msg = msgResult
		if len(data.T) == 0 {
			data.Msg = msgNoresult
		}
	}

	tmplIndex.Execute(w, data)
}

func processMagnetURI(r *http.Request) (string, metainfo.Magnet, error) {
	// Check query
	raw, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		return "", metainfo.Magnet{}, err
	}
	// Parse query
	m, err := metainfo.ParseMagnetUri(raw)
	if err != nil {
		return "", metainfo.Magnet{}, err
	}
	return raw, m, nil
}

func play(w http.ResponseWriter, r *http.Request) {
	streams++ // stats

	// Process magnet URI
	uri, magnet, err := processMagnetURI(r)
	if err != nil {
		log.Println("error magnet uri:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Load torrent
	torrent, err := app.Add(r, magnet)
	if err != nil {
		log.Printf("error loading %s: %v\n", magnet.InfoHash.String(), err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Data to send to play
	var data = struct {
		Title       string
		URI         string
		ContentType string
		Subs        []Subtitle
	}{
		Title:       magnet.DisplayName,
		URI:         uri,
		ContentType: videoMIME(torrent.LargestFile().Path()),
		Subs:        torrent.Subs,
	}

	tmplPlay.Execute(w, data)
}

func stream(w http.ResponseWriter, r *http.Request) {
	// Process magnet URI
	_, magnet, err := processMagnetURI(r)
	if err != nil {
		log.Println("error magnet uri:", err)
		http.Redirect(w, r, "/", 200)
		return
	}

	// Get torrent
	torrent, ok := app.Get(magnet.InfoHash.String())
	if !ok {
		log.Println("error get torrent:", err)
		http.Redirect(w, r, "/", 200)
		return
	}

	// Set activity
	torrent.TrackActivity(r)

	largestFile := torrent.LargestFile()
	reader := largestFile.NewReader()
	reader.SetResponsive()
	defer reader.Close()

	// Set appropriate headers for streaming
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Type", videoMIME(largestFile.Path()))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	// Use ServeContent to handle range requests properly
	http.ServeContent(w, r, torrent.Name(), time.Time{}, reader)
}
