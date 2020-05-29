package main

import (
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"sync"
)

const fileseparator = "_"

var subDB = &SubSite{
	Name:      "SubDB",
	URL:       "api.thesubdb.com", // "sandbox.thesubdb.com"
	UserAgent: "SubDB/1.0 [torrentstream.io] 0.3 beta",
}

// subDBDownload downloads and returns the path of subtitle
func subDBDownload(subtitle Subtitle) (string, error) {
	req, err := http.NewRequest("GET", subtitle.URL, nil)
	if err != nil {
		return "", nil
	}

	req.Header.Set("User-Agent", subDB.UserAgent)
	resp, err := subClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// check if we got any data in responose
	bodystr := string(body)
	if len(bodystr) < 9 {
		return "", errors.New("not found")
	}

	// convert to vtt
	vtt := srt2vtt(string(body))

	// write subfile
	file := subtitle.Hash + fileseparator + subtitle.Lang + ".vtt"
	path := filepath.Join(conf.FileDir, file)
	if err := ioutil.WriteFile(path, []byte(vtt), 0666); err != nil {
		return "", err
	}

	log.Println("subtitle @", path)
	return file, nil
}

func makeURL(hash, lang string) string {
	params := url.Values{}
	params.Set("action", "download")
	params.Set("language", lang)
	params.Set("hash", hash)

	url := url.URL{
		Scheme:   "http",
		Host:     subDB.URL,
		RawQuery: params.Encode(),
	}
	return url.String()
}

func subDBdl(hash string, lang []string) []Subtitle {
	var subs []Subtitle

	var m = &sync.Mutex{}
	var wg sync.WaitGroup
	for _, l := range lang {

		wg.Add(1)
		go func(lang string) {
			defer wg.Done()

			sub := Subtitle{
				Hash:   hash,
				Lang:   lang,
				Format: "vtt",
				URL:    makeURL(hash, lang),
			}

			var err error
			if sub.Path, err = subDB.d(sub); err != nil {
				return
			}

			m.Lock()
			subs = append(subs, sub)
			m.Unlock()
		}(l)
	}

	wg.Wait()
	return subs
}
