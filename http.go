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

// {{range $k, $v := .Ts}}
// {{$v.Conn}}		{{$k}}
// {{end}}

var (
	tplIndex = template.Must(template.ParseFiles("www/index.html"))
	tplPlay  = template.Must(template.ParseFiles("www/play.html"))

	// stats page
	tplStats = template.Must(template.New("stats").Parse(`
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
		Ts      map[string]*T
		History []History
	}{
		Time:    boot,
		Visits:  visits,
		Streams: streams,
		Ts:      ts.m,
		History: hist,
	}

	tplStats.Execute(w, info)
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

	tplIndex.Execute(w, data)
}

func play(w http.ResponseWriter, r *http.Request) {
	streams++ // stats

	// check query
	raw, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		log.Println("error url query:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// parse query
	m, err := metainfo.ParseMagnetURI(raw)
	if err != nil {
		log.Println("error magnet uri:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// load torrent
	t, err := ts.Load(r, m)
	if err != nil {
		log.Printf("error loading %s: %v\n", m.InfoHash.String(), err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// data to send to play
	var data = struct {
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

	tplPlay.Execute(w, data)
}

func stream(w http.ResponseWriter, r *http.Request) {
	// check query
	raw, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		log.Println("error url query:", err)
		http.Redirect(w, r, "/", 200)
		return
	}
	// parse query
	m, err := metainfo.ParseMagnetURI(raw)
	if err != nil {
		log.Println("error magnet uri:", err)
		http.Redirect(w, r, "/", 200)
		return
	}
	// get torrrent
	t, ok := ts.Get(m.InfoHash.String())
	if !ok {
		log.Println("error get torrent:", err)
		http.Redirect(w, r, "/", 200)
		return
	}
	// set activity
	t.activityCtx(r)

	tf := t.Largest()
	tr := tf.NewReader()
	// tr.SetResponsive()	// test without this
	defer tr.Close()

	w.Header().Set("Connection", "keep-alive")
	w.Header().Add("Content-Type", videoMIME(tf.Path()))
	http.ServeContent(w, r, t.Name(), time.Time{}, tr)
}
