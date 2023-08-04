package main

import (
	"encoding/json"
	"io"
	"log"

	"github.com/stinkyfingers/chadedwardsapi/photo"
	"github.com/stinkyfingers/chadedwardsapi/storage"
)

/*
1. Reads files from S# bucket chadedwardsbandphotos
2. Gets exif data from each file
3. Reads metadata from S3 bucket chadedwardsapi/photos
4. Merges exif data with metadata
5. Writes metadata back to S3 bucket chadedwardsapi/photos
*/

func main() {
	store, err := storage.NewS3("jds")
	if err != nil {
		log.Fatal(err)
	}
	exifData, err := getPhotos(store)
	if err != nil {
		log.Fatal(err)
	}
	metaData, err := getPhotoData(store)
	if err != nil {
		log.Fatal(err)
	}
	mergePhotoData(metaData, exifData)
	err = updatePhotoData(store, metaData)
	if err != nil {
		log.Fatal(err)
	}
}

func updatePhotoData(store storage.Storage, photoData map[string]photo.Metadata) error {
	return store.Write(storage.BUCKET_API, storage.KEY_PHOTOS, photoData)
}

func mergePhotoData(photoData map[string]photo.Metadata, exifData map[string]photo.ExifData) error {
	for key, datum := range exifData {
		location, err := datum.GetLocation()
		if err != nil {
			return err
		}

		metaDatum, ok := photoData[key]
		if !ok {
			metaDatum = photo.Metadata{}
		}
		metaDatum.Filename = key
		metaDatum.DateTimeOriginal = datum.DateTimeOriginal
		metaDatum.GPSLatitude = datum.GPSLatitude
		metaDatum.GPSLongitude = datum.GPSLongitude
		metaDatum.Location = location
		photoData[key] = metaDatum
	}
	return nil
}

func getPhotoData(store storage.Storage) (map[string]photo.Metadata, error) {
	data := make(map[string]photo.Metadata)
	r, err := store.Get(storage.BUCKET_API, storage.KEY_PHOTOS)
	if err != nil {
		return nil, err
	}
	err = json.NewDecoder(r).Decode(&data)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return data, nil
}

func getPhotos(store storage.Storage) (map[string]photo.ExifData, error) {
	keys, err := store.List(storage.BUCKET_IMAGES)
	if err != nil {
		return nil, err
	}
	data := make(map[string]photo.ExifData)
	for _, key := range keys {
		log.Print("populating exif data for ", key)
		r, err := store.Get(storage.BUCKET_IMAGES, key)
		if err != nil {
			return nil, err
		}
		datum, err := photo.GetExifData(r)
		if err != nil {
			return nil, err
		}
		data[key] = *datum
	}
	log.Print("got exif data")
	return data, nil
}
