package main

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

var (
	leetx = &TorrentSite{
		Name:      "1337x",
		Scheme:    "https",
		URL:       "1337x.to",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	}
)

func leetxSearch(title, category string, ch chan *Torrent) error {
	defer close(ch)

	// Build search URL
	searchURL := fmt.Sprintf("%s://%s/search/%s/1/", leetx.Scheme, leetx.URL, url.QueryEscape(title))
	if category == "tv" {
		searchURL += "TV/"
	}

	// Create request
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", leetx.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	// Parse response
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return err
	}

	// Define a list of codec indicators to filter out
	unsupportedCodecs := []string{"HEVC", "H.265", "x265", "VP9"}

	var wg sync.WaitGroup
	doc.Find("table.table-list tbody tr").Each(func(i int, s *goquery.Selection) {
		wg.Add(1)
		go func(s *goquery.Selection) {
			defer wg.Done()

			// Extract torrent details
			title := strings.TrimSpace(s.Find("td.name a:nth-child(2)").Text())
			if title == "" {
				return
			}

			// Check if the torrent title contains any of the unsupported codec indicators
			torrentTitleUpper := strings.ToUpper(title)
			for _, codec := range unsupportedCodecs {
				if strings.Contains(torrentTitleUpper, strings.ToUpper(codec)) {
					return
				}
			}

			// Extract size
			size := strings.TrimSpace(s.Find("td.size").Text())

			// Extract seeders and leechers
			seeders, _ := strconv.Atoi(strings.TrimSpace(s.Find("td.seeds").Text()))
			leechers, _ := strconv.Atoi(strings.TrimSpace(s.Find("td.leeches").Text()))

			// Check minimum seeders
			if seeders < conf.Seeders {
				return
			}

			// Get magnet link
			magnetLink, _ := s.Find("td.name a:nth-child(1)").Attr("href")
			if magnetLink == "" {
				return
			}

			// Add trackers if enabled
			if conf.Trackers {
				for _, tracker := range trackers {
					magnetLink += "&tr=" + tracker
				}
			}

			// Create torrent object
			torrent := &Torrent{
				MagnetURI: magnetLink,
				ID:        extractHash(magnetLink),
				Title:     title,
				Size:      size,
				SiteID:    leetx.Name,
				Seeders:   seeders,
				Leechers:  leechers,
			}

			// Get OMDB data
			title, year := parseTitle(torrent.Title)
			omdb, _ := getOMDB(title, year)
			if !strings.Contains(omdb.Poster, "http") {
				omdb.Poster = ""
			}
			torrent.Info = omdb

			ch <- torrent
		}(s)
	})

	wg.Wait()
	return nil
}

// extractHash extracts the info hash from a magnet link
func extractHash(magnet string) string {
	re := regexp.MustCompile(`btih:([a-zA-Z0-9]{40})`)
	matches := re.FindStringSubmatch(magnet)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
