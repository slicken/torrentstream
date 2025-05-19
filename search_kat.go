package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

var kat = &TorrentSite{
	Name:      "Kickass torrents",
	Scheme:    "https",
	URL:       "kickass2.cc",
	UserAgent: "",
}

func katSearch(title, category string, ch chan *Torrent) error {
	defer close(ch)

	// check category
	switch category {
	case "tv":
		category = "tv"
	default:
		category = "movies"
	}

	// create url
	url := url.URL{
		Scheme: kat.Scheme,
		Host:   kat.URL,
		Path:   fmt.Sprintf("/usearch/%s category:%s/", title, category),
	}

	// create request
	req, err := http.NewRequest("GET", url.String(), nil)
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

	// parse
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return err
	}

	// Define a list of codec indicators to filter out
	unsupportedCodecs := []string{"HEVC", "H.265", "x265", "VP9"}

	var wg sync.WaitGroup
	doc.Find("tr#torrent_latest_torrents").Each(func(i int, s *goquery.Selection) {
		if i == 0 {
			return
		}

		// concurrent parse
		wg.Add(1)
		go func() {
			defer wg.Done()

			link, ok := s.Find("a").Eq(1).Attr("href")
			if !ok {
				return
			}
			magnet, err := parseURI(link)
			if err != nil {
				return
			}
			hash, err := parseID(link)
			if err != nil {
				return
			}

			// Extract title
			title := s.Find("div.torrentname a").Text()

			// Check if the torrent title contains any of the unsupported codec indicators
			torrentTitleUpper := strings.ToUpper(title)
			for _, codec := range unsupportedCodecs {
				if strings.Contains(torrentTitleUpper, strings.ToUpper(codec)) {
					return
				}
			}

			// make torrent
			var torrent = new(Torrent)
			torrent.MagnetURI = magnet
			torrent.ID = hash
			torrent.Title = title
			torrent.Seeders, _ = strconv.Atoi(s.Find("td").Eq(3).Text())
			torrent.Leechers, _ = strconv.Atoi(s.Find("td").Eq(1).Text())
			torrent.Size = s.Find("td").Eq(1).Text()
			torrent.SiteID = kat.Name
			// send torrent to channel
			ch <- torrent
		}()
	})

	wg.Wait()
	return nil
}
