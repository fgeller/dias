package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
)

func fail(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

type response struct {
	Path string   `json:"path"`
	Type string   `json:"type"`
	Meta metaData `json:"meta"`
}

type metaData struct {
	Time string `json:"time"`
}

func resetNext(path string) string {
	f, err := os.Open(path)
	fail(err)

	i, err := imaging.Decode(f, imaging.AutoOrientation(true))
	fail(err)

	err = imaging.Save(i, "html/next.jpg")
	fail(err)

	return fmt.Sprintf("/next.jpg?t=%v", time.Now().Unix())
}

func readExif(path string) metaData {
	result := metaData{}

	f, err := os.Open(path)
	defer f.Close()
	fail(err)

	x, err := exif.Decode(f)
	if err != nil {
		log.Printf("failed to decode exif in %#v: %v", path, err)
		return result
	}

	dt, err := x.DateTime()
	if err != nil {
		log.Printf("could not find exif datetime for %#v: %v", path, err)
		fi, err := f.Stat()
		fail(err)
		result.Time = fi.ModTime().Format("2006-01-02")
	} else {
		result.Time = dt.Format("2006-01-02")
	}

	return result
}

func Next(w http.ResponseWriter, r *http.Request) {
	fn := randomString(mediaFiles)
	pth := fmt.Sprintf("html/media/%v", fn)
	x := readExif(pth)
	url := resetNext(pth)

	resp := response{Path: url, Type: "Photo", Meta: x}
	log.Printf("%#v\n", resp)

	enc := json.NewEncoder(w)
	err := enc.Encode(resp)
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
	fmt.Printf("found %v media files.\n", len(mediaFiles))
	http.Handle("/", http.FileServer(http.Dir("html/")))
	http.HandleFunc("/next", Next)
	fail(http.ListenAndServe(":8080", nil))
}
