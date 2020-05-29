package main

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestFunc(t *testing.T) {
	file := "this_is_a_testfile_..mwa"

	ext := filepath.Ext(file)
	fmt.Println(ext)
	fmt.Println(videoMIME(file))
}
