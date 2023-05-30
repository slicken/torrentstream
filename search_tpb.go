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
	UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/53.0.2785.143 Safari/537.36",
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
	req.Header.Set("User-Agent", tpb.UserAgent)
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
			var t = new(Torrent)
			t.MagnetURI = magnet
			t.ID = hash
			t.Title = s.Find("div.detName a").Text()
			t.Seeders, _ = strconv.Atoi(s.Children().Eq(2).Text())
			t.Leechers, _ = strconv.Atoi(s.Children().Eq(3).Text())
			t.Size = s.Find("font.detDesc").Text()
			t.SiteID = tpb.Name

			// send torrent on channel
			ch <- t
		}()
	})

	wg.Wait()
	return nil
}
