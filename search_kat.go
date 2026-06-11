package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var kat = &TorrentSite{
	Name:      "Kickass torrents",
	Scheme:    "https",
	URL:       "kickass2.fun",
	UserAgent: "",
}

func katSearch(title, category string, ch chan *Torrent) error {
	defer close(ch)

	var lastErr error
	for _, host := range torrentSiteHosts(kat.URL, []string{"kickass2.fun", "kat.am", "kickasstorrents.to", "katcr.to"}) {
		results, err := scrapeKATHost(host, title, category, ch)
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

func scrapeKATHost(host, title, category string, ch chan *Torrent) (int, error) {
	total := 0

	for page := 1; page <= maxSearchPages; page++ {
		searchURL := buildKATSearchURL(host, title, category, page)
		doc, err := fetchHTML(searchURL)
		if err != nil {
			if total > 0 {
				break
			}
			return total, err
		}

		pageResults := parseKATResults(doc, ch)
		total += pageResults
		if pageResults == 0 || !hasNextPage(doc) {
			break
		}
	}

	return total, nil
}

func buildKATSearchURL(host, title, category string, page int) string {
	switch category {
	case "tv":
		category = "tv"
	default:
		category = "movies"
	}

	search := fmt.Sprintf("%s category:%s", title, category)
	path := fmt.Sprintf("/usearch/%s/", url.PathEscape(search))
	if page > 1 {
		path = fmt.Sprintf("/usearch/%s/%d/", url.PathEscape(search), page)
	}

	return fmt.Sprintf("%s://%s%s", kat.Scheme, host, path)
}

func parseKATResults(doc *goquery.Document, ch chan *Torrent) int {
	count := 0

	doc.Find("tr").Each(func(i int, s *goquery.Selection) {
		torrent := parseKATRow(s)
		if torrent == nil {
			return
		}
		ch <- torrent
		count++
	})

	return count
}

func parseKATRow(s *goquery.Selection) *Torrent {
	magnet := extractKATMagnet(s)
	if magnet == "" {
		return nil
	}

	hash, err := parseID(magnet)
	if err != nil {
		return nil
	}

	title := strings.TrimSpace(s.Find("a.cellMainLink").First().Text())
	if title == "" {
		title = strings.TrimSpace(s.Find("div.torrentname a").Last().Text())
	}
	if title == "" || hasUnsupportedCodec(title) {
		return nil
	}

	seeders := parseKATNumber(s, "td.green, td.seeders, td:nth-child(4)")
	if conf != nil && seeders < conf.Seeders {
		return nil
	}

	return &Torrent{
		MagnetURI: magnet,
		ID:        hash,
		Title:     title,
		Seeders:   seeders,
		Leechers:  parseKATNumber(s, "td.red, td.leechers, td:nth-child(5)"),
		Size:      parseKATSize(s),
		SiteID:    kat.Name,
	}
}

func parseKATNumber(s *goquery.Selection, selector string) int {
	value, _ := strconv.Atoi(strings.TrimSpace(s.Find(selector).First().Text()))
	return value
}

func parseKATSize(s *goquery.Selection) string {
	size := cleanCellText(s.Find("td.nobr, td.size").First())
	if size != "" {
		return size
	}

	return cleanCellText(s.Find("td").Eq(1))
}

func extractKATMagnet(s *goquery.Selection) string {
	var magnet string
	s.Find("a[href]").EachWithBreak(func(_ int, link *goquery.Selection) bool {
		href, _ := link.Attr("href")
		parsed, err := parseURI(href)
		if err == nil {
			magnet = parsed
			return false
		}
		return true
	})
	return magnet
}
