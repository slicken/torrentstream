package main

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
)

// create a subhash from x kb read of beggining and end of file
func readHash(reader io.ReadSeeker, kb int64) (string, error) {
	var readSize = kb * 1024
	hash := md5.New()

	_, err := io.CopyN(hash, reader, readSize)
	if err != nil {
		return "", err
	}
	_, err = reader.Seek(-readSize, os.SEEK_END)
	if err != nil {
		return "", err
	}
	_, err = io.CopyN(hash, reader, readSize)
	if err != nil {
		return "", err
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
		return "", err
	}

	_, err = f.Seek(-readSize, os.SEEK_END)
	if err != nil {
		return "", err
	}

	_, err = io.CopyN(hash, f, readSize)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// convert and write vtt
func subFileConvert(f string) (string, error) {
	srt, err := ioutil.ReadFile(f)
	if err != nil {
		return "", err
	}

	// convert to vtt
	vtt := srt2vtt(string(srt))

	// write subfile
	file := f + fileseparator + ".vtt"
	path := filepath.Join(conf.FileDir, file)
	if err := ioutil.WriteFile(path, []byte(vtt), 0666); err != nil {
		return "", err
	}

	log.Println("subtitle @", path)
	return file, nil
}

// srt2vtt quick dirty way to convert srt2vtt
func srt2vtt(text string) string {
	srt := regexp.MustCompile(`,`)
	return "WEBVTT\n\n" + srt.ReplaceAllLiteralString(text, `.`)
}
