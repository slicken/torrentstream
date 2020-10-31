package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestTpbParse(t *testing.T) {
	data, err := os.Open("testdata/tpb.html")
	if err != nil {
		t.Fatal("Error opening fixture", err)
	}
	defer data.Close()

	doc, err := goquery.NewDocumentFromReader(data)
	if err != nil {
		t.Fatal("could not read data")
	}

	const siteID = "The Pirate Bay"
	var m sync.Mutex
	var torrents []*Torrent
	doc.Find("table#searchResult tr").Each(func(i int, s *goquery.Selection) {
		if i == 0 {
			return
		}
		magnet, ok := s.Find("div.detName + a").Attr("href")
		if !ok {
			return
		}
		hash, err := parseID(magnet)
		if err != nil {
			log.Print(err)
			return
		}
		title := s.Find("div.detName a").Text()
		seed, _ := strconv.Atoi(s.Children().Eq(2).Text())
		// if seed < conf.MinSeed {
		// 	return
		// }
		leech, _ := strconv.Atoi(s.Children().Eq(3).Text())
		size := s.Find("font.detDesc").Text()

		t := &Torrent{
			MagnetURI: magnet,
			ID:        hash,
			Title:     title,
			SiteID:    siteID,
			Seeders:   seed,
			Leechers:  leech,
			Size:      size,
		}
		m.Lock()
		torrents = append(torrents, t)
		m.Unlock()
	})
	fmt.Println(len(torrents))
}
