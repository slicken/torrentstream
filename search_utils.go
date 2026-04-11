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

// Chrome <video> (no -ffmpeg): filter listings by title heuristics — not exact MIME probing.
var (
	reChromeBadCodec = regexp.MustCompile(`(?i)\b(x265|h\.?265|hevc|hev1|av1|av01)\b`)
	reChromeBadExt   = regexp.MustCompile(`(?i)\.(mkv|avi|wmv|flv|m2ts|mpeg|mpg|vob|divx|ogm|asf|ts)$`)
)

// chromeBrowserLikelyPlayable is false when the release name clearly points at
// codecs or containers Chrome typically will not decode in-browser.
func chromeBrowserLikelyPlayable(title string) bool {
	if reChromeBadCodec.MatchString(title) {
		return false
	}
	if reChromeBadExt.MatchString(strings.TrimSpace(title)) {
		return false
	}
	return true
}

// List of user agents to rotate through (recent Chrome; fewer bot heuristics than ancient UAs).
var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
}

// headerRoundTripper adds browser-like headers (many indexers sit behind CDNs that flag bare Go clients).
type headerRoundTripper struct{ rt http.RoundTripper }

func (h headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ua := GetRandomUserAgent()
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if strings.Contains(ua, "Chrome") && !strings.Contains(ua, "Edg") {
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,application/json;q=0.8,image/avif,image/webp,*/*;q=0.7")
		req.Header.Set("Sec-CH-UA", `"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`)
		req.Header.Set("Sec-CH-UA-Mobile", "?0")
		platform := `"Windows"`
		if strings.Contains(ua, "Linux") {
			platform = `"Linux"`
		} else if strings.Contains(ua, "Macintosh") {
			platform = `"macOS"`
		}
		req.Header.Set("Sec-CH-UA-Platform", platform)
	} else if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "*/*")
	}
	return h.rt.RoundTrip(req)
}

// client for torrent site scraping.
var client = &http.Client{
	Transport: headerRoundTripper{rt: &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
		DisableKeepAlives:   true,
	}},
	Timeout: 25 * time.Second,
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
	case ".webm":
		return "video/webm"
	case ".mkv", ".mka", ".mks":
		return "video/x-matroska"
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
	case ".wma", ".wmv":
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
