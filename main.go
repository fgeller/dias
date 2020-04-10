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

	"github.com/davecgh/go-spew/spew"
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
	Time     string   `json:"time"`
	Location location `json:"location"`
}

type location struct {
	City    string `json:"city"`
	Country string `json:"country"`
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

func readMetaData(path string) metaData {
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

	lat, lon, err := x.LatLong()
	if err == nil {
		url := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?format=jsonv2&lat=%v&lon=%v", lat, lon)
		fmt.Printf(">> url %#v\n", url)
		resp, err := http.Get(url)
		fail(err)

		defer resp.Body.Close()
		var loc nominatimResponse
		err = json.NewDecoder(resp.Body).Decode(&loc)
		fail(err)

		spew.Dump(loc)

		result.Location = location{City: loc.Address.City, Country: loc.Address.Country}
		if result.Location.City == "" {
			result.Location.City = loc.Address.Village
		}

	} else {
		log.Printf(">> no GPS data for %s, err: %v\n", path, err)
	}

	return result
}

type nominatimResponse struct {
	PlaceID     int              `json:"place_id"`
	Licence     string           `json:"licence"`
	OSMType     string           `json:"osm_type"`
	OSMID       int              `json:"osm_id"`
	Lat         string           `json:"lat"`
	Lon         string           `json:"lon"`
	PlaceRank   int              `json:"place_rank"`
	Category    string           `json:"category"`
	Type        string           `json:"type"`
	Importance  float64          `json:"importance"`
	AddressType string           `json:"addresstype"`
	Name        string           `json:"name"`
	DisplayName string           `json:"display_name"`
	Address     nominatimAddress `json:"address"`
	BoundingBox []string         `json:"boundingbox"`
}

type nominatimAddress struct {
	Path        string `json:"path"`
	Suburb      string `json:"suburb"`
	Village     string `json:"village"`
	City        string `json:"city"`
	County      string `json:"county"`
	State       string `json:"state"`
	PostCode    string `json:"postcode"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
}

func Next(w http.ResponseWriter, r *http.Request) {
	fn := randomString(mediaFiles)
	pth := fmt.Sprintf("html/media/%v", fn)
	md := readMetaData(pth)
	url := resetNext(pth)

	resp := response{Path: url, Type: "Photo", Meta: md}
	spew.Dump(resp)

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
