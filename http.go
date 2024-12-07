package main

import (
	"context"
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
	tplIndex = template.Must(template.ParseFiles("www/index.html"))
	tplPlay  = template.Must(template.ParseFiles("www/play.html"))

	// stats page
	tplStats = template.Must(template.New("stats").Parse(`
	Boot    {{.Time}}
	Visits  {{.Visits}}
	Streams {{.Streams}}
	Active  {{.Active}}
	
	--- HISTORY ---
	{{range .History}}
	{{.Time}}	{{.File}}
	{{end}}
	`))

	// stats page
	boot    = time.Now()
	visits  int
	streams int
	hist    []History
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
		Active  int
		History []History
	}{
		Time:    boot,
		Visits:  visits,
		Streams: streams,
		Active:  len(ts.m),
		History: hist,
	}

	tplStats.Execute(w, info)
}

func index(w http.ResponseWriter, r *http.Request) {
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
	} else {
		visits++
	}

	tplIndex.Execute(w, data)
}

func processMagnetURI(r *http.Request) (string, metainfo.Magnet, error) {
	// Check query
	raw, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		return "", metainfo.Magnet{}, err
	}
	// Parse query
	m, err := metainfo.ParseMagnetURI(raw)
	if err != nil {
		return "", metainfo.Magnet{}, err
	}
	return raw, m, nil
}

func play(w http.ResponseWriter, r *http.Request) {
	streams++

	// Process magnet URI
	raw, m, err := processMagnetURI(r)
	if err != nil {
		log.Println("error magnet uri:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Load torrent
	t, err := ts.Load(r, m)
	if err != nil {
		log.Printf("error loading %s: %v\n", m.InfoHash.String(), err)
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
		Title:       m.DisplayName,
		URI:         raw, // Use raw query as URI
		ContentType: videoMIME(t.Largest().Path()),
		Subs:        t.Subs,
	}

	tplPlay.Execute(w, data)
}

func stream(w http.ResponseWriter, r *http.Request) {
	// Process magnet URI
	_, m, err := processMagnetURI(r)
	if err != nil {
		log.Println("error magnet uri:", err)
		http.Redirect(w, r, "/", 200)
		return
	}
	// Get torrent
	t, ok := ts.Get(m.InfoHash.String())
	if !ok {
		log.Println("error get torrent:", err)
		http.Redirect(w, r, "/", 200)
		return
	}
	t.Conn++
	// t.activityCtx(r)
	tFile := t.Largest()
	tReader := tFile.NewReader()
	// tReader.SetResponsive()
	defer tReader.Close()

	ctx, cancel := context.WithTimeout(r.Context(), conf.Idle)
	defer func() {
		t.Conn--
		if t.Conn == 0 {
			tReader.Close()
			cancel()
			if ts.Delete(m.InfoHash.String()) {
				log.Printf("deleted %s (disconnect or timeout)\n", m.DisplayName)
			}
		}
	}()

	// w.Header().Set("Connection", "keep-alive")
	// w.Header().Set("Content-Type", videoMIME(tFile.Path()))
	http.ServeContent(w, r.WithContext(ctx), t.Name(), time.Time{}, tReader)
	// http.ServeContent(w, r, t.Name(), time.Time{}, tReader)
}
