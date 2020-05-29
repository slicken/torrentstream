package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const mb = 16 << 16

var client = &http.Client{
	Transport: &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
		DisableKeepAlives:   true,
		// DisableCompression:  true,
	},
	Timeout: 10 * time.Second,
}

// savetoFile debug store file
func savetoFile(path string, data interface{}) error {
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(data)

	return ioutil.WriteFile(path, b.Bytes(), 0644)
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
