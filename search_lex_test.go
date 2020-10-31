package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestLexParse(t *testing.T) {
	data, err := os.Open("testdata/lex.html")
	if err != nil {
		t.Fatal("Error opening fixture", err)
	}
	defer data.Close()

	doc, err := goquery.NewDocumentFromReader(data)
	if err != nil {
		t.Fatal("could not read data")
	}

	// var torrents []Torrent
	doc.Find("tr").Each(func(i int, s *goquery.Selection) {
		// magnet link
		link, ok := s.Find("a").Eq(1).Attr("href")
		if !ok {
			return
		}
		fmt.Println("link", link)

		// infohash
		// hash, _ := parseID(link) // no links on serach page.. not suitable for this
		// if err != nil {
		// 	fmt.Println("err")
		// 	return
		// }
		// fmt.Println("hash", hash)
		// name
		title := s.Find("td").Eq(0).Find("a").Eq(1).Text()
		fmt.Println("title", title)

		// size
		sizeNode := s.Find("td").Eq(4)
		size := strings.Replace(sizeNode.Text(), sizeNode.Children().Text(), "", -1)
		fmt.Println("size", size)
		// // seeders
		seed, _ := strconv.Atoi(s.Find("td").Eq(2).Text())
		fmt.Println("seed", seed)
		// // leechers
		leech, _ := strconv.Atoi(s.Find("td").Eq(3).Text())
		fmt.Println("leech", leech)
		// movie info
		// omdb, _ := omdbSearch(parseName(title)) //			if omdb.Type != "" && omdb.Type != "movie" {

		// t := Torrent{
		// 	MagnetURI: link,
		// 	ID:        hash,
		// 	Title:     title,
		// 	Size:      size,
		// 	Seeders:   seed,
		// 	Leechers:  leech,
		// 	SiteID:    "leetx",
		// 	// Info:      omdb,
		// }

		// torrents = append(torrents, t)
	})
}
