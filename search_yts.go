package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

var (
	yts = &TorrentSite{
		Name:      "YTS",
		Scheme:    "https",
		URL:       "yts.mx",
		UserAgent: "",
	}
)

// YTSResponse is used only for parsing the API response
type YTSResponse struct {
	Status        string `json:"status"`
	StatusMessage string `json:"status_message"`
	Data          struct {
		MovieCount int `json:"movie_count"`
		Limit      int
		PageNumber int `json:"page_number"`
		Movies     []struct {
			ID       int      `json:"id"`
			URL      string   `json:"url"`
			Title    string   `json:"title"`
			Rating   float32  `json:"rating"`
			Year     int      `json:"year"`
			Runtime  int      `json:"runtime"`
			Summary  string   `json:"summary"`
			Genres   []string `json:"genres"`
			Language string   `json:"language"`
			Torrents []struct {
				Hash    string `json:"hash"`
				Size    string `json:"size"`
				Quality string `json:"quality"`
			} `json:"torrents"`
		} `json:"movies"`
	} `json:"data"`
}

func ytsSearch(title, category string, ch chan *Torrent) error {
	defer close(ch)

	// Create base URL
	baseURL := "https://yts.mx/api/v2/list_movies.json"

	// Build query parameters
	params := url.Values{}
	params.Set("query_term", title)
	params.Set("limit", "50")
	if category == "tv" {
		params.Set("genre", "TV Series")
	}

	// Create request
	apiURL := baseURL + "?" + params.Encode()
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	// Parse response
	var response YTSResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	// Define a list of codec indicators to filter out
	unsupportedCodecs := []string{"HEVC", "H.265", "x265", "VP9"}

	var wg sync.WaitGroup
	for _, movie := range response.Data.Movies {
		wg.Add(1)
		go func(movie struct {
			ID       int      `json:"id"`
			URL      string   `json:"url"`
			Title    string   `json:"title"`
			Rating   float32  `json:"rating"`
			Year     int      `json:"year"`
			Runtime  int      `json:"runtime"`
			Summary  string   `json:"summary"`
			Genres   []string `json:"genres"`
			Language string   `json:"language"`
			Torrents []struct {
				Hash    string `json:"hash"`
				Size    string `json:"size"`
				Quality string `json:"quality"`
			} `json:"torrents"`
		}) {
			defer wg.Done()

			// Find best quality torrent
			var bestTorrent struct {
				Hash    string `json:"hash"`
				Size    string `json:"size"`
				Quality string `json:"quality"`
			}
			// First try to find 1080p
			for _, torrent := range movie.Torrents {
				if torrent.Quality == "1080p" {
					bestTorrent = torrent
					break
				}
			}
			// If no 1080p found, fall back to 720p
			if bestTorrent.Hash == "" {
				for _, torrent := range movie.Torrents {
					if torrent.Quality == "720p" {
						bestTorrent = torrent
						break
					}
				}
			}

			if bestTorrent.Hash == "" {
				return
			}

			// Create magnet link
			escaped := url.QueryEscape(movie.Title)
			spaced := strings.ReplaceAll(escaped, "+", "%20")
			magnet := "magnet:?xt=urn:btih:" + bestTorrent.Hash + "&dn=" + spaced

			// Add trackers if enabled
			if conf.Trackers {
				for _, tracker := range trackers {
					magnet += "&tr=" + tracker
				}
			}

			// Create torrent object using the common Torrent struct
			torrent := &Torrent{
				MagnetURI: magnet,
				ID:        bestTorrent.Hash,
				Title:     fmt.Sprintf("%s (%d)", movie.Title, movie.Year),
				Size:      bestTorrent.Size,
				SiteID:    yts.Name,
				Seeders:   100, // YTS doesn't provide seeder count, using a default high value
				Leechers:  0,   // YTS doesn't provide leecher count
			}

			// Check if the torrent title contains any of the unsupported codec indicators
			torrentTitleUpper := strings.ToUpper(torrent.Title)
			for _, codec := range unsupportedCodecs {
				if strings.Contains(torrentTitleUpper, strings.ToUpper(codec)) {
					return
				}
			}

			// Check minimum seeders
			if torrent.Seeders < conf.Seeders {
				return
			}

			// Get OMDB data
			title, year := parseTitle(torrent.Title)
			omdb, _ := getOMDB(title, year)
			if !strings.Contains(omdb.Poster, "http") {
				omdb.Poster = ""
			}
			torrent.Info = omdb

			ch <- torrent
		}(movie)
	}

	wg.Wait()
	return nil
}
