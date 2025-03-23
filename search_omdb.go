package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/url"
)

const omdbURL = "www.omdbapi.com"

// Omdb (imdb) movie struct
type Omdb struct {
	Title      string `json:"Title"`
	Year       string `json:"Year"`
	Released   string `json:"Released"`
	Runtime    string `json:"Runtime"`
	Genre      string `json:"Genre"`
	Director   string `json:"Director"`
	Actors     string `json:"Actors"`
	Plot       string `json:"Plot"`
	Language   string `json:"Language"`
	Country    string `json:"Country"`
	Poster     string `json:"Poster"`
	Metascore  string `json:"Metascore"`
	ImdbRating string `json:"imdbRating"`
	ImdbVotes  string `json:"imdbVotes"`
	ImdbID     string `json:"imdbID"`
	Type       string `json:"Type"`
	DVD        string `json:"DVD"`
}

// getOMDB ..
func getOMDB(title, year string) (*Omdb, error) {
	var omdb = new(Omdb)

	if OMDB_KEY == "" {
		return omdb, errors.New("OMDB key missing")
	}

	params := url.Values{}
	params.Set("apikey", OMDB_KEY)

	params.Set("t", title)
	if year != "" {
		params.Set("y", year)
	}
	params.Set("r", "json")
	params.Set("plot", "short")

	url := url.URL{
		Scheme:   "https",
		Host:     omdbURL,
		RawQuery: params.Encode(),
	}

	resp, err := client.Get(url.String())
	if err != nil {
		return omdb, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return omdb, err
	}

	if err := json.Unmarshal(body, &omdb); err != nil {
		return omdb, err
	}

	return omdb, nil
}
