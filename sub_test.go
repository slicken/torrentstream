package main

import (
	"fmt"
	"testing"
)

func TestSub(t *testing.T) {
	conf = new(Config)
	conf.FileDir = "./"

	// subhash := "77c8772fc24b1a648885bfe219719d37"
	// subhash := "b3f13973c8c9a28227b2d6ca10c75ce4"
	subhash := "8164f73e9671915560bc9757d8a018ea"

	// init search func
	subDB.search = subDBSearch
	subDB.download = subDBDownload

	// le = append(le, "en")
	// le = append(le, "se")
	// result := subDBdl(subhash, []string{"en", "se"})
	result, err := subDBSearch(subhash, "en")
	if err != nil {
		fmt.Println(err)

	}
	for i, sub := range result {
		fmt.Printf("%d\n%v\n\n", i, sub)
	}
}
