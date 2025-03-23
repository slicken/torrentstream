package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const mb = 16 << 16

// List of user agents to rotate through
var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:89.0) Gecko/20100101 Firefox/89.0",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.107 Safari/537.36",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPad; CPU OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
}

// client with rotating user agent
var client = &http.Client{
	Transport: &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			// Set random user agent for all requests
			req.Header.Set("User-Agent", GetRandomUserAgent())
			return nil, nil
		},
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
		DisableKeepAlives:   true,
		// DisableCompression:  true,
	},
	Timeout: 10 * time.Second,
}

// GetRandomUserAgent returns a random user agent from the list
func GetRandomUserAgent() string {
	return userAgents[time.Now().UnixNano()%int64(len(userAgents))]
}

// savetoFile debug store file
func savetoFile(path string, data interface{}) error {
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(data)

	return os.WriteFile(path, b.Bytes(), 0644)
}

func formatBytes(i int64) string {
	if i >= 1000000000000 {
		return fmt.Sprintf("%.3f TB", float64(i)/1099511627776)
	} else if i >= 1000000000 {
		return fmt.Sprintf("%.2f GB", float64(i)/1073741824)
	} else if i >= 1000000 {
		return fmt.Sprintf("%.1f MB", float64(i)/1048576)
	} else if i >= 1000 {
		return fmt.Sprintf("%.0f KB", float64(i)/1024)
	} else {
		return fmt.Sprintf("%.0f B", float64(i))
	}
}

var (
	reMagnet = regexp.MustCompile(`magnet:(.+)+`)
	reUrn    = regexp.MustCompile(`urn:btih:(.+)+`)
	reYear   = regexp.MustCompile(`((19|20)\d\d)`)
	reEp     = regexp.MustCompile(`[sS]\d{2}[eE]\d+`)
	reFix    = regexp.MustCompile("[^a-zA-Z0-9]+")
)

func parseURI(s string) (string, error) {
	raw, _ := url.QueryUnescape(s)

	res := reMagnet.FindStringSubmatch(raw)
	if len(res) == 0 {
		return "", errors.New("no match")
	}

	return res[0], nil
}

func parseID(s string) (string, error) {
	raw, _ := url.QueryUnescape(s)

	res := reUrn.FindStringSubmatch(raw)
	if len(res) == 0 {
		return "", errors.New("no match")
	}

	return strings.Split(res[1], "&")[0], nil
}

func parseTitle(s string) (title, year string) {
	title = reFix.ReplaceAllString(s, " ")

	e := reEp.FindStringIndex(title)
	y := reYear.FindStringIndex(title)

	if len(y) > 0 {
		year = title[y[0]:y[1]]
	}

	if len(y) > 0 && len(e) > 0 {
		if y[0] < e[0] {
			title = title[:y[0]]
		} else {
			title = title[:e[0]]
		}
	} else if len(y) > 0 {
		title = title[:y[0]]
	} else if len(e) > 0 {
		title = title[:e[0]]
	}

	return
}

func videoMIME(path string) string {
	ext := filepath.Ext(path)

	switch ext {
	case ".m3u", ".m3u8":
		return "application/x-mpegURL"
	case ".flv":
		return "video/x-flv"
	case ".mp4", ".m4a", ".m4p", ".m4b", ".m4r", ".m4v":
		return "video/mp4"
	case ".m1v":
		return "video/mpeg"
	case ".ogg":
		return "video/ogg"
	case ".asf":
		return "video/ms-asf"
	case ".ts":
		return "video/MP2T"
	case ".3gp":
		return "video/3gpp"
	case ".mov", ".qt":
		return "video/quicktime"
	case ".avi":
		return "video/x-msvideo"
	case ".wma", "wmv":
		return "video/x-ms-wmv"
	default:
		return "application/octet-stream"
	}
}

var trackers = []string{
	"udp://open.demonii.com:1337/announce",
	"udp://tracker.openbittorrent.com:80",
	"udp://tracker.coppersurfer.tk:6969",
	"udp://glotorrents.pw:6969/announce",
	"udp://tracker.opentrackr.org:1337/announce",
	"udp://torrent.gresille.org:80/announce",
	"udp://p4p.arenabg.com:1337",
	"udp://tracker.leechers-paradise.org:6969",
}

func addTrackers(magnet string) string {
	if magnet == "" {
		return magnet
	}

	for _, tracker := range trackers {
		magnet += "&tr=" + tracker
	}

	return magnet
}
