package main

import (
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Torrent is a TorrentSite search results
type Torrent struct {
	MagnetURI string // magnet link
	ID        string // info hash
	Title     string // torrent name
	Size      string
	SiteID    string
	Seeders   int
	Leechers  int
	Info      *Omdb
}

// TorrentSite struct
type TorrentSite struct {
	Name       string
	Scheme     string
	URL        string
	UserAgent  string
	Enabled    bool
	ScrapeFunc func(string, string, chan *Torrent) error
}

// TorrentSites contains torrent search sites
type TorrentSites struct {
	sites []*TorrentSite
	sync.RWMutex
}

var (
	cachedMovies []*Torrent
	cachedMutex  sync.RWMutex
)

// InitializeSearch initializes the search variable with all torrent sites
func InitializeSearch() *TorrentSites {
	// meta-search on external torrent sites
	tpb.ScrapeFunc = tpbSearch
	kat.ScrapeFunc = katSearch
	yts.ScrapeFunc = ytsSearch
	leetx.ScrapeFunc = leetxSearch

	ts := &TorrentSites{
		sites: []*TorrentSite{
			tpb,
			kat,
			yts,
			leetx,
		},
	}

	// Initialize all sites as enabled by default
	for _, site := range ts.sites {
		site.Enabled = true
	}

	return ts
}

// List ..
func (ts *TorrentSites) List() []*TorrentSite {
	ts.RLock()
	defer ts.RUnlock()

	return ts.sites
}

// Enabled ..
func (ts *TorrentSites) Enabled() []*TorrentSite {
	ts.RLock()
	defer ts.RUnlock()

	var sites []*TorrentSite
	for _, site := range ts.sites {
		if site.Enabled {
			sites = append(sites, site)
		}
	}

	return sites
}

// siteHealthURL requests a path that should return 200 when the indexer is usable.
// Hitting only "/" often yields 403 from CDNs while real search/API endpoints work.
func siteHealthURL(site *TorrentSite) string {
	u := url.URL{Scheme: site.Scheme, Host: site.URL}
	switch site.Name {
	case "1337x":
		u.Path = "/search/2020/1/"
	case "Kickass torrents":
		u.Path = "/usearch/2020/"
	case "YTS":
		u.Path = "/api/v2/list_movies.json"
		q := url.Values{}
		q.Set("limit", "1")
		u.RawQuery = q.Encode()
	default:
		u.Path = "/"
	}
	return u.String()
}

// Handler is keeping track of torrentSites, if they are up or down.
func (ts *TorrentSites) Handler(minutes int) {
	go func() {
		for {
			for i, site := range ts.List() {
				req, err := http.NewRequest("GET", siteHealthURL(site), nil)
				if err != nil {
					log.Println(site.Name, "health URL:", err)
					continue
				}

				resp, err := client.Do(req)
				if err != nil {
					ts.sites[i].Enabled = false
					log.Println(site.Name, "Disabled:", err)
					continue
				}
				resp.Body.Close()

				switch {
				case resp.StatusCode == http.StatusOK:
					ts.sites[i].Enabled = true
				case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized:
					// Cloudflare and similar often return 403 to non-browser TLS; do not mark dead.
					ts.sites[i].Enabled = true
					log.Printf("%s health check: %d (CDN/bot filter; leaving enabled — searches may still work)", site.Name, resp.StatusCode)
				default:
					ts.sites[i].Enabled = false
					log.Println(site.Name, "Disabled - Status code:", resp.StatusCode)
				}
			}

			time.Sleep(time.Duration(minutes) * time.Minute)
		}
	}()
}

// SearchTorrent makes concurrent search on all enabled sites..
func (ts *TorrentSites) SearchTorrent(title, category string) []*Torrent {
	var wg sync.WaitGroup
	var mapOmdb = make(map[string]*Omdb, 0)
	var ch = make(chan *Torrent)

	for _, site := range ts.Enabled() {
		wg.Add(1)
		go func(site TorrentSite) {
			defer wg.Done()

			var c = make(chan *Torrent)
			go func() {
				err := site.ScrapeFunc(title, category, c)
				if err != nil {
					log.Printf("search error '%s' on %s: %v\n", title, site.Name, err)
				}
			}()

			// get omdb content
			var vg sync.WaitGroup
			for torrent := range c {
				vg.Add(1)
				go func(torrent *Torrent, mapOmdb map[string]*Omdb) {
					defer vg.Done()

					// check is this torrent nr of seeders passes minimum seeders in config
					if torrent.Seeders < conf.Seeders {
						return
					}
					if conf != nil && !conf.FFmpeg && !chromeBrowserLikelyPlayable(torrent.Title) {
						return
					}
					title, year := parseTitle(torrent.Title)

					// check if poster alredy exist from previous dl
					ts.RLock()
					list := mapOmdb
					ts.RUnlock()

					for name, omdb := range list {
						if name == title {
							torrent.Info = omdb
							ch <- torrent
							return
						}
					}
					// if info didnt exist, download it
					omdb, _ := getOMDB(title, year)
					// if no link to poster. clear its field to endable the 'no poster' imgage
					if !strings.Contains(omdb.Poster, "http") {
						omdb.Poster = ""
					}
					torrent.Info = omdb
					ch <- torrent
					ts.Lock()
					mapOmdb[title] = omdb
					ts.Unlock()
				}(torrent, mapOmdb)
			}
			vg.Wait()
		}(*site)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	// add to slice
	var torrents = make([]*Torrent, 0)
	for torrent := range ch {
		torrents = append(torrents, torrent)
	}

	// sort by seeders
	sort.SliceStable(torrents, func(i, j int) bool {
		return torrents[i].Seeders > torrents[j].Seeders
	})

	return torrents
}

// InitializeMovieList performs an initial search across all sites and caches the results
func InitializeMovieList() {
	// Initialize search if not already initialized
	if search == nil {
		search = InitializeSearch()
		search.Handler(conf.Sites)
		log.Printf("initialized torrent sites for %s, %s, %s, %s\n", tpb.Name, kat.Name, yts.Name, leetx.Name)
	}

	// Initial cache population
	updateMoviesCache()

	// Start daily update goroutine
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				updateMoviesCache()
			}
		}
	}()
}

// updateMoviesCache updates the cached movies from all sites
func updateMoviesCache() {
	searchText := strconv.Itoa(time.Now().Year())
	movies := search.SearchTorrent(searchText, "")

	cachedMutex.Lock()
	cachedMovies = movies
	cachedMutex.Unlock()
	log.Printf("Updated cashed movie list with %d movies (search %s)", len(movies), searchText)
}

// GetCachedMovies returns the cached movies
func GetCachedMovies() []*Torrent {
	cachedMutex.RLock()
	defer cachedMutex.RUnlock()
	return cachedMovies
}
