package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// create a subhash from x kb read of beggining and end of file
func readHash(reader io.ReadSeeker, kb int64) (string, error) {
	var readSize = kb * 1024
	hash := md5.New()

	_, err := io.CopyN(hash, reader, readSize)
	if err != nil {
		return "", fmt.Errorf("io.CopyN error: %s", err)
	}
	_, err = reader.Seek(-readSize, io.SeekEnd)
	if err != nil {
		return "", fmt.Errorf("read.Seek error: %s", err)
	}
	_, err = io.CopyN(hash, reader, readSize)
	if err != nil {
		return "", fmt.Errorf("io.CopyN2 error: %s", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// create a subhash from x kb read of beggining and end of file
func subHash(path string, kb int64) (string, error) {
	var readSize = kb * 1024
	hash := md5.New()
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = io.CopyN(hash, f, readSize)
	if err != nil {
		return "", fmt.Errorf("io.CopyN error: %s", err)
	}
	_, err = f.Seek(-readSize, io.SeekEnd)
	if err != nil {
		return "", fmt.Errorf("read.Seek error: %s", err)
	}
	_, err = io.CopyN(hash, f, readSize)
	if err != nil {
		return "", fmt.Errorf("io.CopyN2 error: %s", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// convert and write vtt
func subFileConvert(f string) (string, error) {
	srt, err := os.ReadFile(f)
	if err != nil {
		return "", err
	}
	// convert to vtt
	vtt := srt2vtt(string(srt))
	// write subfile
	file := f + ".vtt"
	if err := os.WriteFile(file, []byte(vtt), 0666); err != nil {
		return "", err
	}

	return file, nil
}

// srt2vtt quick dirty way to convert srt2vtt
func srt2vtt(text string) string {
	srt := regexp.MustCompile(`,`)
	return "WEBVTT\n\n" + srt.ReplaceAllLiteralString(text, `.`)
}

// detectLanguageFromFilename tries to extract language code from subtitle filename
func detectLanguageFromFilename(fileName string) string {
	fileName = strings.ToLower(fileName)

	// Common language patterns in subtitle files
	langPatterns := map[string]string{
		".en.":        "en",
		".eng.":       "en",
		"english.":    "en",
		".fr.":        "fr",
		".fre.":       "fr",
		"french.":     "fr",
		".es.":        "es",
		".spa.":       "es",
		"spanish.":    "es",
		".de.":        "de",
		".ger.":       "de",
		"german.":     "de",
		".it.":        "it",
		".ita.":       "it",
		"italian.":    "it",
		".pt.":        "pt",
		".por.":       "pt",
		"portuguese.": "pt",
		".ru.":        "ru",
		".rus.":       "ru",
		"russian.":    "ru",
		".se.":        "se",
		".swe.":       "se",
		"swedish.":    "se",
		".nl.":        "nl",
		".dut.":       "nl",
		"dutch.":      "nl",
		".ja.":        "ja",
		".jpn.":       "ja",
		"japanese.":   "ja",
		".zh.":        "zh",
		".chi.":       "zh",
		"chinese.":    "zh",
		".ko.":        "ko",
		".kor.":       "ko",
		"korean.":     "ko",
	}

	for pattern, lang := range langPatterns {
		if strings.Contains(fileName, pattern) {
			return lang
		}
	}

	return "unknown"
}
