package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestListIDsReturnsTorrentMapKeys(t *testing.T) {
	app := &App{
		torrents: map[string]*T{
			"info-hash": {ID: "torrent-id"},
		},
	}

	got := app.ListIDs()
	want := []string{"info-hash"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListIDs() = %v, want %v", got, want)
	}
}

func TestIdleForRequiresNoConnectionsAndExpiredActivity(t *testing.T) {
	torrent := &T{
		Conn:     0,
		Activity: time.Now().Add(-(torrentIdleTimeout + time.Minute)),
	}
	if !torrent.IdleFor(torrentIdleTimeout) {
		t.Fatal("expected torrent to be idle after timeout with no connections")
	}

	torrent.Conn = 1
	if torrent.IdleFor(torrentIdleTimeout) {
		t.Fatal("expected active connection to prevent idle cleanup")
	}

	torrent.Conn = 0
	torrent.Activity = time.Now()
	if torrent.IdleFor(torrentIdleTimeout) {
		t.Fatal("expected recent activity to prevent idle cleanup")
	}
}

func TestTouchRefreshesTorrentActivity(t *testing.T) {
	const magnetURL = "/touch?magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=test"

	req := httptest.NewRequest(http.MethodPost, magnetURL, nil)
	_, magnet, err := processMagnetURI(req)
	if err != nil {
		t.Fatal(err)
	}

	oldApp := app
	t.Cleanup(func() { app = oldApp })

	tracked := &T{Activity: time.Now().Add(-time.Hour)}
	app = &App{
		torrents: map[string]*T{
			magnet.InfoHash.String(): tracked,
		},
	}

	rec := httptest.NewRecorder()
	touch(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("touch status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if time.Since(tracked.Activity) > time.Second {
		t.Fatal("expected touch to refresh torrent activity")
	}
}

func Test1337SearchFollowsDetailPagesAndPaginates(t *testing.T) {
	const (
		hash1 = "0123456789abcdef0123456789abcdef01234567"
		hash2 = "abcdef0123456789abcdef0123456789abcdef01"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/category-search/2026/Movies/1/":
			w.Write([]byte(`
				<table class="table-list"><tbody>
					<tr>
						<td class="name"><a href="/sub/1">icon</a><a href="/torrent/1/open-movie-2026/">Open Movie 2026 1080p</a></td>
						<td class="seeds">34</td><td class="leeches">5</td><td class="size">1.4 GB<span>1 day</span></td>
					</tr>
					<tr>
						<td class="name"><a href="/sub/2">icon</a><a href="/torrent/2/skip-h265/">Skip H265 2026 x265</a></td>
						<td class="seeds">99</td><td class="leeches">1</td><td class="size">2 GB</td>
					</tr>
				</tbody></table>
				<ul class="pagination"><li class="next"><a href="/category-search/2026/Movies/2/">Next</a></li></ul>`))
		case "/category-search/2026/Movies/2/":
			w.Write([]byte(`
				<table class="table-list"><tbody>
					<tr>
						<td class="name"><a href="/sub/3">icon</a><a href="/torrent/3/second-movie-2026/">Second Movie 2026 720p</a></td>
						<td class="seeds">12</td><td class="leeches">2</td><td class="size">900 MB</td>
					</tr>
				</tbody></table>`))
		case "/torrent/1/open-movie-2026/":
			w.Write([]byte(`<a href="magnet:?xt=urn:btih:` + hash1 + `&dn=open">Magnet Download</a>`))
		case "/torrent/3/second-movie-2026/":
			w.Write([]byte(`<a href="magnet:?xt=urn:btih:` + hash2 + `&dn=second">Magnet Download</a>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldScheme, oldURL, oldConf := leetx.Scheme, leetx.URL, conf
	t.Cleanup(func() {
		leetx.Scheme, leetx.URL, conf = oldScheme, oldURL, oldConf
	})
	leetx.Scheme = "http"
	leetx.URL = strings.TrimPrefix(server.URL, "http://")
	conf = &Config{Seeders: 1, Trackers: false}

	results, err := collectSearchResults(func(ch chan *Torrent) error {
		return leetxSearch("2026", "movie", ch)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("leetxSearch results = %d, want 2", len(results))
	}
	if results[0].ID != hash1 || results[1].ID != hash2 {
		t.Fatalf("leetxSearch IDs = %q, %q; want %q, %q", results[0].ID, results[1].ID, hash1, hash2)
	}
}

func TestKATSearchReadsPaginatedMagnetRows(t *testing.T) {
	const (
		hash1 = "1111111111111111111111111111111111111111"
		hash2 = "2222222222222222222222222222222222222222"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/usearch/") && strings.HasSuffix(r.URL.Path, "/2/") {
			w.Write([]byte(katSearchPage("Second KAT Movie 2026", hash2, "800 MB", 7, 1, false)))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/usearch/") {
			w.Write([]byte(katSearchPage("First KAT Movie 2026", hash1, "1.1 GB", 21, 4, true)))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	oldScheme, oldURL, oldConf := kat.Scheme, kat.URL, conf
	t.Cleanup(func() {
		kat.Scheme, kat.URL, conf = oldScheme, oldURL, oldConf
	})
	kat.Scheme = "http"
	kat.URL = strings.TrimPrefix(server.URL, "http://")
	conf = &Config{Seeders: 1, Trackers: false}

	results, err := collectSearchResults(func(ch chan *Torrent) error {
		return katSearch("2026", "movie", ch)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("katSearch results = %d, want 2", len(results))
	}
	if results[0].Title != "First KAT Movie 2026" || results[1].Title != "Second KAT Movie 2026" {
		t.Fatalf("katSearch titles = %q, %q", results[0].Title, results[1].Title)
	}
}

func TestSearchURLBuildersDoNotDoubleEscapeTerms(t *testing.T) {
	old1337Scheme, oldKATScheme := leetx.Scheme, kat.Scheme
	t.Cleanup(func() {
		leetx.Scheme, kat.Scheme = old1337Scheme, oldKATScheme
	})
	leetx.Scheme = "https"
	kat.Scheme = "https"

	got1337 := build1337SearchURL("1337.example", "open movie", "movie", 2)
	want1337 := "https://1337.example/category-search/open%20movie/Movies/2/"
	if got1337 != want1337 {
		t.Fatalf("build1337SearchURL() = %q, want %q", got1337, want1337)
	}

	gotKAT := buildKATSearchURL("kat.example", "open movie", "movie", 2)
	wantKAT := "https://kat.example/usearch/open%20movie%20category:movies/2/"
	if gotKAT != wantKAT {
		t.Fatalf("buildKATSearchURL() = %q, want %q", gotKAT, wantKAT)
	}
}

func TestStreamTorrentFiltersAndStreamsResults(t *testing.T) {
	oldConf := conf
	t.Cleanup(func() { conf = oldConf })
	conf = &Config{Seeders: 5, FFmpeg: false}

	ts := &TorrentSites{sites: []*TorrentSite{{
		Name:    "fixture",
		Enabled: true,
		ScrapeFunc: func(title, category string, ch chan *Torrent) error {
			defer close(ch)
			ch <- &Torrent{Title: "slow result 2026 mp4", Seeders: 1}
			ch <- &Torrent{Title: "good result 2026 mp4", Seeders: 12}
			ch <- &Torrent{Title: "bad codec 2026 x265", Seeders: 50}
			return nil
		},
	}}}

	var results []*Torrent
	for torrent := range ts.StreamTorrent("2026", "movie") {
		results = append(results, torrent)
	}

	if len(results) != 1 {
		t.Fatalf("StreamTorrent results = %d, want 1", len(results))
	}
	if results[0].Title != "good result 2026 mp4" {
		t.Fatalf("StreamTorrent result title = %q", results[0].Title)
	}
}

func TestSearchStreamEmitsResultAndDoneEvents(t *testing.T) {
	oldSearch, oldConf := search, conf
	t.Cleanup(func() {
		search, conf = oldSearch, oldConf
	})
	conf = &Config{Seeders: 1, FFmpeg: true}
	search = &TorrentSites{sites: []*TorrentSite{{
		Name:    "fixture",
		Enabled: true,
		ScrapeFunc: func(title, category string, ch chan *Torrent) error {
			defer close(ch)
			ch <- &Torrent{
				MagnetURI: "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=test",
				ID:        "0123456789abcdef0123456789abcdef01234567",
				Title:     "streamed result",
				Size:      "1 GB",
				SiteID:    "fixture",
				Seeders:   10,
				Leechers:  2,
				Info: &Omdb{
					Title:      "Streamed Result",
					Year:       "2026",
					Runtime:    "90 min",
					Genre:      "Action",
					Poster:     "https://example.com/poster.jpg",
					Metascore:  "75",
					ImdbRating: "7.1",
					Plot:       "A streamed search result.",
					Director:   "Test Director",
					Actors:     "Test Actor",
				},
			}
			return nil
		},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/search?search=2026&category=movie", nil)
	rec := httptest.NewRecorder()
	searchStream(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("searchStream status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: result") {
		t.Fatalf("searchStream body missing result event: %s", body)
	}
	if !strings.Contains(body, `"title":"streamed result"`) {
		t.Fatalf("searchStream body missing torrent payload: %s", body)
	}
	if !strings.Contains(body, `"poster":"https://example.com/poster.jpg"`) {
		t.Fatalf("searchStream body missing enriched info: %s", body)
	}
	if !strings.Contains(body, "event: done") || !strings.Contains(body, `"count":1`) {
		t.Fatalf("searchStream body missing done count: %s", body)
	}
}

func TestLiveTorrentMirrorSmoke(t *testing.T) {
	if os.Getenv("LIVE_SCRAPE") != "1" {
		t.Skip("set LIVE_SCRAPE=1 to hit live torrent mirrors")
	}

	oldConf := conf
	t.Cleanup(func() { conf = oldConf })
	conf = &Config{Seeders: 0, Trackers: false}

	tests := []struct {
		name     string
		query    string
		searchFn func(string, string, chan *Torrent) error
	}{
		{name: "1337x", query: "Mortal Kombat II 2026", searchFn: leetxSearch},
		{name: "Kickass", query: "2026", searchFn: katSearch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := collectSearchResults(func(ch chan *Torrent) error {
				return tt.searchFn(tt.query, "movie", ch)
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(results) == 0 {
				t.Fatalf("%s returned no live results for %q", tt.name, tt.query)
			}
			t.Logf("%s returned %d live results; first: %s", tt.name, len(results), results[0].Title)
		})
	}
}

func katSearchPage(title, hash, size string, seeders, leechers int, next bool) string {
	nextLink := ""
	if next {
		nextLink = `<a href="/usearch/2026%20category%3Amovies/2/">Next</a>`
	}
	magnet := "magnet:?xt=urn:btih:" + hash + "&dn=test"
	wrappedMagnet := "https://mylink.cloud/?url=" + url.QueryEscape(magnet)

	return `
		<table><tbody>
			<tr id="torrent_latest_torrents">
				<td><div class="torrentname"><a class="cellMainLink" href="/torrent/1/">` + title + `</a></div></td>
				<td class="nobr">` + size + `</td>
				<td>files</td>
				<td class="green">` + strconv.Itoa(seeders) + `</td>
				<td class="red">` + strconv.Itoa(leechers) + `</td>
				<td><a href="` + wrappedMagnet + `">magnet</a></td>
			</tr>
		</tbody></table>` + nextLink
}

func collectSearchResults(search func(chan *Torrent) error) ([]*Torrent, error) {
	ch := make(chan *Torrent)
	errCh := make(chan error, 1)
	go func() {
		errCh <- search(ch)
	}()

	var results []*Torrent
	for torrent := range ch {
		results = append(results, torrent)
	}

	return results, <-errCh
}
