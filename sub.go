package main

import (
	"net"
	"net/http"
	"time"
)

// subClient is httpClient for requesting subtitles
var subClient = &http.Client{
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

// Subtitle type for SubDB
type Subtitle struct {
	Hash   string
	Lang   string
	Format string
	Path   string
	URL    string
}

// SubSite struct
type SubSite struct {
	Name      string
	URL       string
	UserAgent string
	Enabled   bool
	search    func(string, string) ([]Subtitle, error)
	download  func(Subtitle) (string, error)
}
