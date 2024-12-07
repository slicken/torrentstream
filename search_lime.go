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

var lime = &TorrentSite{
	Name:      "Lime torrents",
	Scheme:    "https",
	URL:       "www.limetorrents.lol",
	UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/53.0.2785.143 Safari/537.36",
}

func limeSearch(title, category string, ch chan *Torrent) error {
	defer close(ch)

	// check category
	switch category {
	case "tv":
		category = "tv"
	default:
		category = "movies"
	}

	// create URL

	fixTitle := strings.ReplaceAll(title, " ", "-")
	url := url.URL{
		Scheme: lime.Scheme,
		Host:   lime.URL,
		Path:   fmt.Sprintf("/search/%s/%s/", category, fixTitle),
	}

	// create request
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", lime.UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	// parse document
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	doc.Find("table.table2 tbody tr").Each(func(i int, s *goquery.Selection) {
		if i == 0 {
			return
		}

		// concurrent parse
		wg.Add(1)
		go func() {
			defer wg.Done()

			title := s.Find("td div.tt-name a").Text()
			link, ok := s.Find("a.csprite_dl14").Attr("href")
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

			seeders, _ := strconv.Atoi(s.Find("td:nth-child(3)").Text())
			leechers, _ := strconv.Atoi(s.Find("td:nth-child(4)").Text())
			size := s.Find("td:nth-child(2)").Text()

			// make torrent
			var t = new(Torrent)
			t.MagnetURI = magnet
			t.ID = hash
			t.Title = title
			t.Seeders = seeders
			t.Leechers = leechers
			t.Size = size
			t.SiteID = lime.Name

			// send torrent to channel
			ch <- t
		}()
	})

	wg.Wait()
	return nil
}
