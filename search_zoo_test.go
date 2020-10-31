package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"sync"
	"testing"
)

func TestZooParse(t *testing.T) {
	data, err := os.Open("testdata/zoo2.xml")
	if err != nil {
		t.Fatal("Error opening fixture", err)
	}
	defer data.Close()

	const siteID = "Zooqle"

	type ZooItem struct {
		MagnetURI string `xml:"torrent>magneturi"`
		ID        string `xml:"torrent>infohash"`
		Title     string `xml:"title"`
		Seeders   int    `xml:"seeds"`
		Leechers  int    `xml:"peers"`
		Size      int64  `xml:"torrent>contentlength"`
	}

	var d struct {
		Items []ZooItem `xml:"channel>item"`
	}

	if err := xml.NewDecoder(data).Decode(&d); err != nil {
		t.Fatal("Error decode", err)
	}

	// torrents := make([]Torrent, len(d.Items))
	// for i, item := range d.Items {
	// 	uri, err := parseURI(item.MagnetURI)
	// 	if err != nil {
	// 		continue
	// 	}
	// 	torrents[i] = Torrent{
	// 		MagnetURI: uri,
	// 		ID:        item.ID,
	// 		Title:     item.Title,
	// 		SiteID:    siteID,
	// 		Seeders:   item.Seeders,
	// 		Leechers:  item.Leechers,
	// 		Size:      formatBytes(item.Size),
	// 	}
	// }

	var m sync.Mutex
	var wg sync.WaitGroup

	var torrents []Torrent
	for _, item := range d.Items {

		wg.Add(1)
		go func(item *ZooItem) {
			defer wg.Done()

			uri, err := parseURI(item.MagnetURI)
			if err != nil {
				return
			}
			// omdb, _ := omdbSearch(parseTitle(item.Title)) //			if omdb.Type != "" && omdb.Type != "movie" {

			t := Torrent{
				MagnetURI: uri,
				ID:        item.ID,
				Title:     item.Title,
				SiteID:    siteID,
				Seeders:   item.Seeders,
				Leechers:  item.Leechers,
				Size:      formatBytes(item.Size),
				// Info:      omdb,
			}
			m.Lock()
			torrents = append(torrents, t)
			m.Unlock()
		}(&item)
	}
	wg.Wait()

	fmt.Println("tor", len(torrents))
}
