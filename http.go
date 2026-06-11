package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"sync"
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

type searchResultEvent struct {
	MagnetURI string           `json:"magnetURI"`
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Size      string           `json:"size"`
	SiteID    string           `json:"siteID"`
	Seeders   int              `json:"seeders"`
	Leechers  int              `json:"leechers"`
	Info      *searchInfoEvent `json:"info,omitempty"`
}

type searchInfoEvent struct {
	Title      string `json:"title"`
	Year       string `json:"year"`
	Runtime    string `json:"runtime"`
	Genre      string `json:"genre"`
	Poster     string `json:"poster"`
	Metascore  string `json:"metascore"`
	ImdbRating string `json:"imdbRating"`
	Plot       string `json:"plot"`
	Director   string `json:"director"`
	Actors     string `json:"actors"`
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

	// Get cached movies by default
	data.T = GetCachedMovies()

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

func searchStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if search == nil {
		http.Error(w, "search is not initialized", http.StatusServiceUnavailable)
		return
	}

	query := r.URL.Query().Get("search")
	category := r.URL.Query().Get("category")
	if query == "" {
		http.Error(w, "missing search query", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	mapOmdb := make(map[string]*Omdb)
	var omdbMutex sync.Mutex
	count := 0
	for torrent := range search.StreamTorrent(query, category) {
		select {
		case <-r.Context().Done():
			return
		default:
		}

		enrichTorrentInfo(torrent, mapOmdb, &omdbMutex)
		payload, err := json.Marshal(searchResultEvent{
			MagnetURI: torrent.MagnetURI,
			ID:        torrent.ID,
			Title:     torrent.Title,
			Size:      torrent.Size,
			SiteID:    torrent.SiteID,
			Seeders:   torrent.Seeders,
			Leechers:  torrent.Leechers,
			Info:      newSearchInfoEvent(torrent.Info),
		})
		if err != nil {
			log.Println("search stream encode:", err)
			continue
		}

		fmt.Fprintf(w, "event: result\ndata: %s\n\n", payload)
		flusher.Flush()
		count++
	}

	fmt.Fprintf(w, "event: done\ndata: {\"count\":%d}\n\n", count)
	flusher.Flush()
}

func newSearchInfoEvent(info *Omdb) *searchInfoEvent {
	if info == nil {
		return nil
	}
	return &searchInfoEvent{
		Title:      info.Title,
		Year:       info.Year,
		Runtime:    info.Runtime,
		Genre:      info.Genre,
		Poster:     info.Poster,
		Metascore:  info.Metascore,
		ImdbRating: info.ImdbRating,
		Plot:       info.Plot,
		Director:   info.Director,
		Actors:     info.Actors,
	}
}

// requestBaseURL returns the client-facing origin (scheme + host) for building absolute links.
func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fp := r.Header.Get("X-Forwarded-Proto"); fp == "https" || fp == "http" {
		scheme = fp
	}
	host := r.Host
	if host == "" {
		host = "127.0.0.1"
	}
	return scheme + "://" + host
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
		if err.Error() == "maximum number of active streams reached. Please wait for some streams to finish." {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	streamEndpoint := "stream"
	largestFile := torrent.LargestFile()
	if largestFile == nil {
		http.Error(w, "torrent contains no streamable files", http.StatusNotFound)
		return
	}
	contentType := videoMIME(largestFile.Path())
	if conf.FFmpeg {
		streamEndpoint = "ffmpeg"
		contentType = "video/mp4"
	}

	// Full stream URL for <source>; use template.URL so html/template does not
	// mangle RawQuery (magnet links contain ?, &, :, etc.).
	playSrc := "/" + streamEndpoint + "?" + r.URL.RawQuery
	keepAliveURL := "/touch?" + r.URL.RawQuery
	fallbackStreamURL := requestBaseURL(r) + playSrc

	// Data to send to play
	var data = struct {
		Title             string
		URI               string
		StreamEndpoint    string
		PlaySrc           template.URL
		KeepAliveURL      template.URL
		FallbackStreamURL string
		FFmpegMode        bool
		ContentType       string
		Subs              []Subtitle
	}{
		Title:             magnet.DisplayName,
		URI:               uri,
		StreamEndpoint:    streamEndpoint,
		PlaySrc:           template.URL(playSrc),
		KeepAliveURL:      template.URL(keepAliveURL),
		FallbackStreamURL: fallbackStreamURL,
		FFmpegMode:        conf.FFmpeg,
		ContentType:       contentType,
		Subs:              torrent.Subtitles(),
	}

	tmplPlay.Execute(w, data)
}

func touch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_, magnet, err := processMagnetURI(r)
	if err != nil {
		log.Println("touch magnet uri:", err)
		http.Error(w, "bad magnet uri", http.StatusBadRequest)
		return
	}

	torrent, ok := app.Get(magnet.InfoHash.String())
	if !ok {
		http.Error(w, "torrent not found", http.StatusNotFound)
		return
	}

	torrent.Touch()
	w.WriteHeader(http.StatusNoContent)
}

func stream(w http.ResponseWriter, r *http.Request) {
	// Process magnet URI
	_, magnet, err := processMagnetURI(r)
	if err != nil {
		log.Println("error magnet uri:", err)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Get torrent
	torrent, ok := app.Get(magnet.InfoHash.String())
	if !ok {
		log.Printf("torrent not found for infoHash: %s\n", magnet.InfoHash.String())
		http.Error(w, "torrent not found", http.StatusNotFound)
		return
	}

	// Track connection activity
	torrent.TrackConnectionActivity(r)

	largestFile := torrent.LargestFile()
	if largestFile == nil {
		http.Error(w, "torrent contains no streamable files", http.StatusNotFound)
		return
	}
	reader := largestFile.NewReader()
	reader.SetReadahead(32 * 1024 * 1024) // 32MB readahead for smooth streaming
	// SetResponsive() triggers anacrolix/torrent internal checkPendingPiecesMatchesRequestOrder
	// panics (piece request order vs _pendingPieces desync) on some releases — omit until upstream fix.
	defer reader.Close()

	// Set streaming headers
	w.Header().Set("Content-Type", videoMIME(largestFile.Path()))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Use ServeContent to handle range requests properly
	http.ServeContent(w, r, torrent.Name(), time.Time{}, reader)
}
