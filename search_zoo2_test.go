package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestZoo2Parse(t *testing.T) {
	data, err := os.Open("testdata/zoo.html")
	if err != nil {
		t.Fatal("Error opening fixture", err)
	}
	defer data.Close()

	doc, err := goquery.NewDocumentFromReader(data)
	if err != nil {
		t.Fatal("could not read data")
	}

	doc.Find("div").Each(func(i int, s *goquery.Selection) {
		// if i == 0 {
		// 	return
		// }

		m, _ := s.Find("li a").Attr("href")
		fmt.Println(m)
		// magnet, ok := s.Find("a").Attr("href")
		// if !ok {
		// 	return
		// }
		// fmt.Println(magnet)
		// magnet, ok := s.Find("div.detName + a").Attr("href")
		// if !ok {
		// 	return
		// }
		// hash, err := parseID(magnet)
		// if err != nil {
		// 	log.Print(err)
		// 	return
		// }
		// title := s.Find("div.detName a").Text()
		// seed, _ := strconv.Atoi(s.Children().Eq(2).Text())
		// // if seed < conf.MinSeed {
		// // 	return
		// // }
		// leech, _ := strconv.Atoi(s.Children().Eq(3).Text())
		// size := s.Find("font.detDesc").Text()

		// t := &Torrent{
		// 	MagnetURI: magnet,
		// 	ID:        hash,
		// 	Title:     title,
		// 	Seeders:   seed,
		// 	Leechers:  leech,
		// 	Size:      size,
		// }
		// fmt.Println(t)
	})
}
