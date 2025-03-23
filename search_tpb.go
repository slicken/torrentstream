package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

var tpb = &TorrentSite{
	Name:      "The Piratebay",
	Scheme:    "https",
	URL:       "thepiratebay0.org",
	UserAgent: "",
}

func tpbSearch(title, category string, ch chan *Torrent) error {
	defer close(ch)

	// check category
	switch category {
	case "tv":
		category = "205"
	default:
		category = "200"
	}

	// create url
	url := url.URL{
		Scheme: kat.Scheme,
		Host:   tpb.URL,
		Path:   fmt.Sprintf("search/%s/0/99/%s", title, category),
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

	// parse request
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	doc.Find("table#searchResult tr").Each(func(i int, s *goquery.Selection) {
		if i == 0 {
			return
		}

		// concurrent parse
		wg.Add(1)
		go func() {
			defer wg.Done()

			magnet, ok := s.Find("div.detName + a").Attr("href")
			if !ok {
				return
			}
			hash, err := parseID(magnet)
			if err != nil {
				log.Print(err)
				return
			}

			// make torrent
			var torrent = new(Torrent)
			torrent.MagnetURI = magnet
			torrent.ID = hash
			torrent.Title = s.Find("div.detName a").Text()
			torrent.Seeders, _ = strconv.Atoi(s.Children().Eq(2).Text())
			torrent.Leechers, _ = strconv.Atoi(s.Children().Eq(3).Text())
			torrent.Size = s.Find("font.detDesc").Text()
			torrent.SiteID = tpb.Name

			// send torrent on channel
			ch <- torrent
		}()
	})

	wg.Wait()
	return nil
}
