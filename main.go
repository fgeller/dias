package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	"github.com/jdeng/goheif"
	"github.com/rwcarlsen/goexif/exif"
)

func fail(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func warn(err error) {
	if err != nil {
		log.Println(err)
	}
}

func randomize(strs []string) {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	rnd.Shuffle(len(strs), func(i, j int) { tmp := strs[i]; strs[i] = strs[j]; strs[j] = tmp })
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

func (s *server) refreshNextVideo(path string, md metaData) string {
	buf, err := ioutil.ReadFile(path)
	fail(err)

	target := filepath.Join(s.htmlDir, "next.mov")
	s.lock.Lock()
	err = ioutil.WriteFile(target, buf, 0755)
	s.lock.Unlock()
	fail(err)

	return "next.mov"
}

func (s *server) refreshNextJPG(path string, md metaData) string {
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

	err = imaging.Save(i, filepath.Join(s.htmlDir, "next.jpg"))
	fail(err)

	return "next.jpg"
}

func (s *server) refreshNextPNG(path string, md metaData) string {
	f, err := os.Open(path)
	fail(err)

	i, err := imaging.Decode(f)
	fail(err)

	err = imaging.Save(i, filepath.Join(s.htmlDir, "next.png"))
	fail(err)

	return "next.png"
}

func (s *server) refreshNextPhoto(path string, md metaData) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return s.refreshNextPNG(path, md)
	default:
		return s.refreshNextJPG(path, md)
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

func (s *server) readVideoMetaData(path string) metaData {
	result := metaData{}

	f, err := os.Open(path)
	defer f.Close()
	fail(err)

	fi, err := f.Stat()
	fail(err)

	result.Time = fi.ModTime().Format("2006-01-02")

	return result
}

func (s *server) readPhotoMetaData(path string) metaData {
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

type config struct {
	HTMLDir  string
	MediaDir string
	Addr     string
}

func readFlags() (config, error) {
	result := config{}
	flag.StringVar(&result.HTMLDir, "html-dir", "", "Directory that contains html and will be served")
	flag.StringVar(&result.MediaDir, "media-dir", "", "Directory that contains media files")
	flag.StringVar(&result.Addr, "addr", ":8080", "Adress that the server will listen at")
	flag.Parse()

	if result.HTMLDir == "" {
		return result, fmt.Errorf("html-dir is required")
	}

	if result.MediaDir == "" {
		return result, fmt.Errorf("media-dir is required")
	}

	return result, nil
}

type server struct {
	htmlDir    string
	mediaDir   string
	addr       string
	mediaFiles []string

	lock *sync.Mutex
	mux  *http.ServeMux
}

func newServer(addr, htmlDir, mediaDir string) *server {
	return &server{
		addr:     addr,
		htmlDir:  htmlDir,
		mediaDir: mediaDir,
	}
}

func (s *server) findMedia() ([]string, error) {
	result := []string{}
	walker := func(pth string, info os.FileInfo, err error) error {
		switch strings.ToLower(filepath.Ext(info.Name())) {
		case ".heic", ".jpg", ".jpeg", ".png":
			result = append(result, pth)
		}
		return err
	}
	err := filepath.Walk(s.mediaDir, walker)

	log.Printf("found %v media files in %v, err=%v\n", len(result), s.mediaDir, err)
	return result, err
}

func (s *server) refreshMedia() {
	var err error

	s.mediaFiles, err = s.findMedia()
	fail(err)

	if len(s.mediaFiles) == 0 {
		fail(fmt.Errorf("media files are required"))
	}

	randomize(s.mediaFiles)
}

func (s *server) takeNextMediaFile() string {
	if len(s.mediaFiles) == 0 {
		s.refreshMedia()
	}

	next := s.mediaFiles[0]
	s.mediaFiles = s.mediaFiles[1:]

	return next
}

func (s *server) next(w http.ResponseWriter, r *http.Request) {
	fp := s.takeNextMediaFile()
	var resp response

	if isVideo(fp) {
		resp.Type = "Video"
		resp.Meta = s.readVideoMetaData(fp)
		resp.Path = s.refreshNextVideo(fp, resp.Meta)
	} else {
		resp.Type = "Photo"
		resp.Meta = s.readPhotoMetaData(fp)
		resp.Path = s.refreshNextPhoto(fp, resp.Meta)
	}

	err := json.NewEncoder(w).Encode(resp)
	warn(err)
}

func (s *server) start() error {
	s.refreshMedia() // early feedback on missing files

	s.mux = http.NewServeMux()
	s.mux.Handle("/", http.FileServer(http.Dir(s.htmlDir)))
	s.mux.HandleFunc("/next", s.next)

	log.Printf("starting server at %#v serving %#v", s.addr, s.htmlDir)
	return http.ListenAndServe(s.addr, s.mux)
}

func main() {
	cfg, err := readFlags()
	fail(err)

	err = newServer(cfg.Addr, cfg.HTMLDir, cfg.MediaDir).start()
	fail(err)
}
