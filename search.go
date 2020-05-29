package main

import (
	"log"
	"net/http"
	"net/url"
	"sort"
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
	Name      string
	Scheme    string
	URL       string
	UserAgent string
	Enabled   bool
	f         func(string, string, chan *Torrent) error
}

// TorrentSites contains torrent search sites
type TorrentSites struct {
	sites []*TorrentSite
	sync.RWMutex
}

// List ..
func (db *TorrentSites) List() []*TorrentSite {
	db.RLock()
	defer db.RUnlock()

	return db.sites
}

// Enabled ..
func (db *TorrentSites) Enabled() []*TorrentSite {
	db.RLock()
	defer db.RUnlock()

	var sites []*TorrentSite
	for _, site := range db.sites {
		if site.Enabled {
			sites = append(sites, site)
		}
	}

	return sites
}

// Handler is Enabling Disablid external search sites based on site response
func (db *TorrentSites) Handler(minutes int) {

	for {
		for i, site := range db.List() {

			// make url
			url := url.URL{
				Scheme: site.Scheme,
				Host:   site.URL,
			}

			req, err := http.NewRequest("GET", url.String(), nil)
			if err != nil {
				return
			}
			// do request
			req.Header.Set("User-Agent", site.UserAgent)
			resp, err := client.Do(req)
			if err != nil {
				db.sites[i].Enabled = false
				log.Println(site.Name, "Disabled:", err)
				continue
			}
			resp.Body.Close()

			// update status
			if resp.StatusCode != http.StatusOK {
				db.sites[i].Enabled = false
				log.Println(site.Name, "Disabled - Status code:", resp.StatusCode)
			} else {
				db.sites[i].Enabled = true
			}
		}

		time.Sleep(time.Duration(minutes) * time.Minute)
	}
}

// SearchTorrent makes concurrent search on all enabled sites..
func (db *TorrentSites) SearchTorrent(title, category string) []*Torrent {
	var wg sync.WaitGroup
	var infos = make(map[string]*Omdb, 0)
	var ch = make(chan *Torrent)

	for _, s := range db.Enabled() {

		wg.Add(1)
		go func(site TorrentSite) {
			defer wg.Done()

			var c = make(chan *Torrent)
			go func() {
				err := site.f(title, category, c)
				if err != nil {
					log.Printf("search error '%s' on %s: %v\n", title, site.Name, err)
				}
			}()

			// get omdb content
			var vg sync.WaitGroup
			for t := range c {

				vg.Add(1)
				go func(t *Torrent, infos map[string]*Omdb) {
					defer vg.Done()

					// check is this torrent nr of seeders passes minimum seeders in config
					if t.Seeders < conf.Seeders {
						return
					}
					title, year := parseTitle(t.Title)
					// check if poster alredy exist from previous dl
					db.RLock()
					list := infos
					db.RUnlock()
					for k, omdb := range list {
						if k == title {
							t.Info = omdb
							ch <- t
							return
						}
					}
					// if info didnt exist, dl
					omdb, _ := omdbGet(title, year)
					// if no link to poster. clear its field to endable "no poster" img
					if !strings.Contains(omdb.Poster, "http") {
						omdb.Poster = ""
					}
					t.Info = omdb
					ch <- t
					db.Lock()
					infos[title] = omdb
					db.Unlock()
				}(t, infos)
			}
			vg.Wait()
		}(*s)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	// add to slice
	var torrents = make([]*Torrent, 0)
	for t := range ch {
		torrents = append(torrents, t)
	}

	// sort by seeders
	sort.SliceStable(torrents, func(i, j int) bool {
		return torrents[i].Seeders > torrents[j].Seeders
	})

	return torrents
}
