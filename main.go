package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/jdeng/goheif"
	"github.com/rwcarlsen/goexif/exif"
)

var mediaFiles []string

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
	Time     string   `json:"time"`
	Location location `json:"location"`

	exif *exif.Exif
}

type location struct {
	Village string `json:"village"`
	City    string `json:"city"`
	Country string `json:"country"`
}

// inline disintegration/imaging to support heif images
func fixOrientation(img image.Image, orientation string) image.Image {
	switch orientation {
	case "1":
	case "2":
		return imaging.FlipH(img)
	case "3":
		return imaging.Rotate180(img)
	case "4":
		return imaging.FlipV(img)
	case "5":
		return imaging.Transpose(img)
	case "6":
		return imaging.Rotate270(img)
	case "7":
		return imaging.Transverse(img)
	case "8":
		return imaging.Rotate90(img)
	}
	return img
}

func refreshNextVideo(path string, md metaData) string {
	buf, err := ioutil.ReadFile(path)
	fail(err)

	target := filepath.Join("html", "next.mov")
	err = ioutil.WriteFile(target, buf, 0755)
	fail(err)

	return "/next.mov"
}

func refreshNextJPG(path string, md metaData) string {
	f, err := os.Open(path)
	fail(err)

	i, err := imaging.Decode(f)
	fail(err)

	if md.exif != nil {
		tg, err := md.exif.Get(exif.Orientation)
		if err != nil {
			log.Printf("failed to read orientation for %#v: %v", path, err)
		} else {
			i = fixOrientation(i, tg.String())
		}
	}

	err = imaging.Save(i, "html/next.jpg")
	fail(err)

	return "/next.jpg"
}

func refreshNextPNG(path string, md metaData) string {
	f, err := os.Open(path)
	fail(err)

	i, err := imaging.Decode(f)
	fail(err)

	err = imaging.Save(i, "html/next.png")
	fail(err)

	return "/next.png"
}

func refreshNextPhoto(path string, md metaData) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return refreshNextPNG(path, md)
	default:
		return refreshNextJPG(path, md)
	}
}

func isHEIF(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".heic":
		return true
	default:
		return false
	}
}

func isVideo(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".mov":
		return true
	default:
		return false
	}
}

func readVideoMetaData(path string) metaData {
	result := metaData{}

	f, err := os.Open(path)
	defer f.Close()
	fail(err)

	fi, err := f.Stat()
	fail(err)

	result.Time = fi.ModTime().Format("2006-01-02")

	return result
}

func readPhotoMetaData(path string) metaData {
	result := metaData{}

	f, err := os.Open(path)
	defer f.Close()
	fail(err)

	if isHEIF(path) {
		buf, err := goheif.ExtractExif(f)
		fail(err)
		result.exif, err = exif.Decode(bytes.NewReader(buf))
	} else {
		result.exif, err = exif.Decode(f)
	}

	if err != nil {
		log.Printf("warning: failed to decode exif in %#v: %v", path, err)
		return result
	}

	dt, err := result.exif.DateTime()
	if err != nil {
		log.Printf("could not find exif datetime for %#v: %v", path, err)
		fi, err := f.Stat()
		fail(err)
		result.Time = fi.ModTime().Format("2006-01-02")
	} else {
		result.Time = dt.Format("2006-01-02")
	}

	lat, lon, err := result.exif.LatLong()
	if err == nil {
		url := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?format=jsonv2&lat=%v&lon=%v", lat, lon)
		resp, err := http.Get(url)
		fail(err)

		defer resp.Body.Close()
		var loc nominatimResponse
		err = json.NewDecoder(resp.Body).Decode(&loc)
		fail(err)
		log.Printf("location display name: %#v\n", loc.DisplayName)

		result.Location = location{
			Village: loc.Address.Village,
			City:    loc.Address.City,
			Country: loc.Address.Country,
		}

	} else {
		log.Printf("found no gps data for %s, err: %v\n", path, err)
	}

	return result
}

func Next(w http.ResponseWriter, r *http.Request) {
	fn := randomString(mediaFiles)
	pth := fmt.Sprintf("html/media/%v", fn)
	var resp response

	if isVideo(pth) {
		resp.Type = "Video"
		resp.Meta = readVideoMetaData(pth)
		resp.Path = refreshNextVideo(pth, resp.Meta)
	} else {
		resp.Type = "Photo"
		resp.Meta = readPhotoMetaData(pth)
		resp.Path = refreshNextPhoto(pth, resp.Meta)
	}

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
		case ".heic", ".jpg", ".jpeg", ".png":
			result = append(result, info.Name())
			// case ".mov":
			// 	result = append(result, info.Name())
			// TODO png
		}
		return err
	}
	err := filepath.Walk("html/media", walker)
	return result, err
}

func main() {
	var err error
	mediaFiles, err = findMedia()
	fail(err)
	log.Printf("found %v media files.\n", len(mediaFiles))

	http.Handle("/", http.FileServer(http.Dir("html/")))
	http.HandleFunc("/next", Next)

	addr := ":8080"
	log.Printf("starting server at %#v", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
