package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func fail(err error) {
	if err != nil {
		panic(err)
	}
}

type response struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

func Next(w http.ResponseWriter, r *http.Request) {
	mf := fmt.Sprintf("/media/%s", randomString(mediaFiles))
	fmt.Printf(">> next: %v\n", mf)
	enc := json.NewEncoder(w)
	err := enc.Encode(response{Path: mf, Type: "Photo"})
	fail(err)
}

func randomString(strs []string) string {
	rnd := rand.New(rand.NewSource(time.Now().Unix()))
	return strs[rnd.Intn(len(strs))]
}

func findMedia() ([]string, error) {
	result := []string{}
	walker := func(pth string, info os.FileInfo, err error) error {
		switch strings.ToLower(filepath.Ext(info.Name())) {
		case ".jpg":
			result = append(result, info.Name())
		}
		return err
	}
	err := filepath.Walk("html/media", walker)
	return result, err
}

var mediaFiles []string

func main() {
	var err error
	mediaFiles, err = findMedia()
	fail(err)
	fmt.Printf("found %v media files.", len(mediaFiles))
	http.Handle("/", http.FileServer(http.Dir("html/")))
	http.HandleFunc("/next", Next)
	fail(http.ListenAndServe(":8080", nil))
}
