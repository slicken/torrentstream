package main

import (
	"fmt"
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
	raw, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		return "", metainfo.Magnet{}, fmt.Errorf("failed to unescape query: %w", err)
	}
	m, err := metainfo.ParseMagnetUri(raw)
	if err != nil {
		return "", metainfo.Magnet{}, fmt.Errorf("failed to parse magnet URI: %w", err)
	}
	return raw, m, nil
}

func play(w http.ResponseWriter, r *http.Request) {
	streams++

	raw, m, err := processMagnetURI(r)
	if err != nil {
		log.Println("error magnet uri:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t, err := ts.Load(r, m)
	if err != nil {
		log.Printf("error loading %s: %v\n", m.InfoHash.String(), err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Title       string
		URI         string
		ContentType string
		Subs        []Subtitle
	}{
		Title:       m.DisplayName,
		URI:         raw,
		ContentType: videoMIME(t.Largest().Path()),
		Subs:        t.Subs,
	}

	log.Println("content type:", data.ContentType)
	tplPlay.Execute(w, data)
}

func stream(w http.ResponseWriter, r *http.Request) {
	log.Printf("stream() - start - URL: %s, Method: %s, RemoteAddr: %s", r.URL.String(), r.Method, r.RemoteAddr)

	_, m, err := processMagnetURI(r)
	if err != nil {
		log.Println("error magnet uri:", err)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	t, ok := ts.Get(m.InfoHash.String())
	if !ok {
		log.Println("error get torrent:", err)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Use the request context for activityCtx
	t.activityCtx(r.Context())

	tf := t.Largest()
	tr := tf.NewReader()
	tr.SetResponsive()
	defer tr.Close()

	w.Header().Set("Connection", "keep-alive")
	w.Header().Add("Content-Type", videoMIME(tf.Path()))
	http.ServeContent(w, r, t.Name(), time.Time{}, tr)

	log.Println("stream() - end")
}
