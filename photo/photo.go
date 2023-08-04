package photo

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
)

type Metadata struct {
	Filename         string    `json:"filename"`
	DateTimeOriginal time.Time `json:"datetimeOriginal"`
	GPSLongitude     float64   `json:"gpsLongitude"`
	GPSLatitude      float64   `json:"gpsLatitude"`
	Location         *Location `json:"location"`
	Category         string    `json:"category"`
	Tags             string    `json:"tags"`
}

type ExifData struct {
	DateTimeOriginal time.Time
	GPSLongitude     float64
	GPSLatitude      float64
}

type PositionStackResponse struct {
	Data []Location `json:"data"`
}

type Location struct {
	Label       string  `json:"label"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Number      string  `json:"number"`
	Street      string  `json:"street"`
	PostalCode  string  `json:"postal_code"`
	Region      string  `json:"region"`
	RegionCode  string  `json:"region_code"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	MapURL      string  `json:"map_url"`
	Confidence  float64 `json:"confidence"`
}

type GooglePhotoRequest struct {
	Url      string   `json:"url"`
	Filename string   `json:"filename"`
	MimeType string   `json:"mimeType"`
	ID       string   `json:"id"`
	Metadata Metadata `json:"metadata"`
}

// Skip Writer for exif writing
type writerSkipper struct {
	w           io.Writer
	bytesToSkip int
}

const (
	positionStackEndpoint = "http://api.positionstack.com/v1/"
)

func GetExifData(r io.Reader) (*ExifData, error) {
	exif.RegisterParsers(mknote.All...)
	x, err := exif.Decode(r)
	if err != nil {
		return nil, err
	}
	datetime, err := x.DateTime()
	if err != nil {
		log.Println("error getting datetime", err)
	}
	lat, long, err := x.LatLong()
	if err != nil {
		log.Println("error getting latlong", err)
	}

	return &ExifData{
		DateTimeOriginal: datetime,
		GPSLongitude:     long,
		GPSLatitude:      lat,
	}, nil
}

func (e *ExifData) GetLocation() (*Location, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%sreverse?access_key=%s&query=%f,%f", positionStackEndpoint, os.Getenv("POSITIONSTACK_KEY"), e.GPSLatitude, e.GPSLongitude), nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var positionStackResponse PositionStackResponse
	err = json.NewDecoder(resp.Body).Decode(&positionStackResponse)
	if err != nil {
		return nil, err
	}
	return BestLocation(positionStackResponse.Data), nil
}

func BestLocation(locations []Location) *Location {
	if len(locations) == 0 {
		return nil
	}

	best := locations[0]
	for _, location := range locations {
		if location.Confidence > best.Confidence {
			best = location
		}
	}
	return &best
}

func CreateThumbnail(name string) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}
	dst := imaging.Thumbnail(src, 100, 100, imaging.CatmullRom)
	tmp, err := os.CreateTemp("", "photo.*.jpeg")
	if err != nil {
		return "", err
	}

	err = imaging.Encode(tmp, dst, imaging.JPEG)
	if err != nil {
		return "", err
	}
	return tmp.Name(), nil
}

func GetGooglePhoto(request GooglePhotoRequest) (*os.File, error) {
	cli := &http.Client{}
	req, err := http.NewRequest("GET", request.Url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	tmp, err := os.CreateTemp("", request.Filename)
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(tmp, resp.Body)
	if err != nil {
		return nil, err
	}
	return tmp, nil
}

func PngToJpg(name string, w io.WriteSeeker) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		return err
	}
	img := image.NewRGBA(src.Bounds())
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(img, img.Bounds(), src, src.Bounds().Min, draw.Over)
	tmp, err := os.CreateTemp("", "photo.*.jpeg")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	opt := &jpeg.Options{
		Quality: 90,
	}
	err = jpeg.Encode(tmp, img, opt)
	if err != nil {
		return err
	}
	_, err = w.Seek(0, 0)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, tmp)
	return err
}

func UpdateMetadata(src, dst map[string]Metadata) {
	for k, v := range src {
		dst[k] = v
	}
}
