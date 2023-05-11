package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

var kat = &TorrentSite{
	Name:      "Kickass torrents",
	Scheme:    "https",
	URL:       "kickass2.cc",
	UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/53.0.2785.143 Safari/537.36",
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
	req.Header.Set("User-Agent", kat.UserAgent)
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
			// make torrent
			var t = new(Torrent)
			t.MagnetURI = magnet
			t.ID = hash
			t.Title = s.Find("div.torrentname a").Text()
			t.Seeders, _ = strconv.Atoi(s.Find("td").Eq(3).Text())
			t.Leechers, _ = strconv.Atoi(s.Find("td").Eq(1).Text())
			t.Size = s.Find("td").Eq(1).Text()
			t.SiteID = kat.Name
			// send torrent to channel
			ch <- t
		}()
	})

	wg.Wait()
	return nil
}
