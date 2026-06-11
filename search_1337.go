package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const maxSearchPages = 10

var (
	leetx = &TorrentSite{
		Name:      "1337x",
		Scheme:    "https",
		URL:       "www.1337xx.to",
		UserAgent: "",
	}
)

func leetxSearch(title, category string, ch chan *Torrent) error {
	defer close(ch)

	var lastErr error
	for _, host := range torrentSiteHosts(leetx.URL, []string{"www.1337xx.to", "1337xx.to", "1337x.to", "1337x.st", "x1337x.ws"}) {
		results, err := scrape1337Host(host, title, category, ch)
		if err == nil {
			return nil
		}
		lastErr = err
		if results > 0 {
			return nil
		}
	}
	return lastErr
}

func scrape1337Host(host, title, category string, ch chan *Torrent) (int, error) {
	total := 0
	var lastErr error

	for page := 1; page <= maxSearchPages; page++ {
		searchURL := build1337SearchURL(host, title, category, page)
		doc, err := fetchHTML(searchURL)
		if err != nil {
			if total > 0 {
				break
			}
			return total, err
		}

		pageResults := parse1337Results(host, doc, ch)
		total += pageResults
		if pageResults == 0 || !hasNextPage(doc) {
			break
		}
		lastErr = nil
	}

	return total, lastErr
}

func build1337SearchURL(host, title, category string, page int) string {
	escapedTitle := url.PathEscape(title)
	path := fmt.Sprintf("/category-search/%s/Movies/%d/", escapedTitle, page)
	if category == "tv" {
		path = fmt.Sprintf("/category-search/%s/TV/%d/", escapedTitle, page)
	}

	return fmt.Sprintf("%s://%s%s", leetx.Scheme, host, path)
}

func parse1337Results(host string, doc *goquery.Document, ch chan *Torrent) int {
	count := 0

	doc.Find("table.table-list tbody tr").Each(func(i int, s *goquery.Selection) {
		result := parse1337Row(host, s)
		if result == nil {
			return
		}
		ch <- result
		count++
	})

	return count
}

func parse1337Row(host string, s *goquery.Selection) *Torrent {
	titleLink := s.Find("td.name a").Last()
	title := strings.TrimSpace(titleLink.Text())
	if title == "" || hasUnsupportedCodec(title) {
		return nil
	}

	seeders, _ := strconv.Atoi(strings.TrimSpace(s.Find("td.seeds, td.coll-2").First().Text()))
	if conf != nil && seeders < conf.Seeders {
		return nil
	}

	detailPath, ok := titleLink.Attr("href")
	if !ok || detailPath == "" {
		return nil
	}

	magnetLink, err := fetch1337Magnet(host, detailPath)
	if err != nil || magnetLink == "" {
		return nil
	}
	if conf != nil && conf.Trackers {
		magnetLink = addTrackers(magnetLink)
	}
	hash, _ := parseID(magnetLink)

	return &Torrent{
		MagnetURI: magnetLink,
		ID:        hash,
		Title:     title,
		Size:      cleanCellText(s.Find("td.size, td.coll-4").First()),
		SiteID:    leetx.Name,
		Seeders:   seeders,
		Leechers:  parseIntCell(s.Find("td.leeches, td.coll-3").First()),
	}
}

func fetch1337Magnet(host, detailPath string) (string, error) {
	detailURL := absoluteSiteURL(leetx.Scheme, host, detailPath)
	doc, err := fetchHTML(detailURL)
	if err != nil {
		return "", err
	}

	magnet, ok := doc.Find(`a[href^="magnet:"]`).First().Attr("href")
	if !ok {
		return "", fmt.Errorf("magnet not found")
	}
	return magnet, nil
}
