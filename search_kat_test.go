package main

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestKatParse(t *testing.T) {
	data, err := os.Open("testdata/kat.html")
	if err != nil {
		t.Fatal("Error opening fixture", err)
	}
	defer data.Close()

	doc, err := goquery.NewDocumentFromReader(data)
	if err != nil {
		t.Fatal("could not read data")
	}

	doc.Find("tr#torrent_latest_torrents").Each(func(i int, s *goquery.Selection) {
		if i == 0 {
			return
		}
		// magnet link
		link, ok := s.Find("a").Eq(1).Attr("href")
		if !ok {
			return
		}
		uri, err := parseURI(link)
		if err != nil {
			return
		}
		fmt.Println("magnet", uri)

		// infohash
		hash, err := parseID(uri)
		if err != nil {
			return
		}
		fmt.Println("id", hash)
		// name
		title := s.Find("div.torrentname a").Text()
		fmt.Println("title", title)
		// size
		size := s.Find("td").Eq(1).Text()
		fmt.Println("size", size)
		// seeders
		seed, _ := strconv.Atoi(s.Find("td").Eq(3).Text())
		fmt.Println("seeders", seed)
		// leechers
		leech, _ := strconv.Atoi(s.Find("td").Eq(1).Text())
		fmt.Println("leechers", leech)
		// movie info
	})
}
