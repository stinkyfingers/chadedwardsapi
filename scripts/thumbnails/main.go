package main

import (
	"log"
	"os"

	"github.com/stinkyfingers/chadedwardsapi/photo"
	"github.com/stinkyfingers/chadedwardsapi/storage"
)

/*
Creates thumbnails for all images in the images bucket, skipping thumbs that already exist.
*/

func main() {
	store, err := storage.NewS3("jds")
	if err != nil {
		log.Fatal(err)
	}
	names, err := getPhotoNames(store)
	if err != nil {
		log.Fatal(err)
	}
	thumbnailNames, err := getThumbnailNames(store)
	if err != nil {
		log.Fatal(err)
	}
	for _, name := range names {
		if thumbnailNames[name] != struct{}{} {
			continue
		}
		err = convertAndUploadPhoto(name, store)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func getPhotoNames(store storage.Storage) ([]string, error) {
	return store.List(storage.BUCKET_IMAGES)
}

func getThumbnailNames(store storage.Storage) (map[string]struct{}, error) {
	names, err := store.List(storage.BUCKET_THUMBNAILS)
	if err != nil {
		return nil, err
	}
	m := make(map[string]struct{})
	for _, name := range names {
		m[name] = struct{}{}
	}
	return m, nil
}

func convertAndUploadPhoto(key string, store storage.Storage) error {
	r, err := store.Get(storage.BUCKET_IMAGES, key)
	if err != nil {
		return err
	}
	defer r.Close()

	tmp, err := os.CreateTemp("", "")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	err = photo.CreateThumbnail(r, tmp)
	if err != nil {
		return err
	}
	tmp.Close()

	err = store.Upload(storage.BUCKET_THUMBNAILS, key, tmp.Name())
	return err

}
