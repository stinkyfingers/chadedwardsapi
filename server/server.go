package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/stinkyfingers/chadedwardsapi/auth"
	"github.com/stinkyfingers/chadedwardsapi/email"
	"github.com/stinkyfingers/chadedwardsapi/photo"
	"github.com/stinkyfingers/chadedwardsapi/request"
	"github.com/stinkyfingers/chadedwardsapi/sms"
	"github.com/stinkyfingers/chadedwardsapi/storage"
)

type Server struct {
	Storage storage.Storage
	SMS     sms.SMS
}

type Suggestion struct {
	Message string `json:"message"`
	Song    string `json:"song"`
	Artist  string `json:"artist"`
}

type Permission map[string]time.Time // ip:time

var (
	timeout = time.Minute * 10
)

func NewServer(profile string) (*Server, error) {
	storage, err := storage.NewS3(profile)
	if err != nil {
		return nil, err
	}

	return &Server{
		Storage: storage,
		SMS:     sms.NewNexmo(),
	}, nil
}

// NewMux returns the router
func NewMux(s *Server) (http.Handler, error) {
	gcp := &auth.GCP{}
	mux := http.NewServeMux()
	mux.Handle("/requests", cors(s.HandleListRequests))
	mux.Handle("/request", cors(s.HandlePostRequest))
	mux.Handle("/auth", cors(gcp.Middleware(status)))            // route to test auth
	mux.Handle("/test", cors(gcp.Middleware(s.HandleProtected))) // route to test auth
	mux.Handle("/photos/list", cors(s.HandleListPhotos))
	mux.Handle("/photos/update", cors(gcp.Middleware(s.HandleUpdatePhotos)))
	mux.Handle("/photos/upload", cors(gcp.Middleware(s.HandleUploadPhotos)))
	mux.Handle("/health", cors(status))
	return mux, nil
}

func isPermittedOrigin(origin string) string {
	var permittedOrigins = []string{
		"http://localhost:3000",
		"https://chadedwardsband.com",
		"https://www.chadedwardsband.com",
		"http://localhost:3001",
	}
	for _, permittedOrigin := range permittedOrigins {
		if permittedOrigin == origin {
			return origin
		}
	}
	return ""
}

func cors(handler http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		permittedOrigin := isPermittedOrigin(r.Header.Get("Origin"))
		w.Header().Set("Access-Control-Allow-Origin", permittedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		if r.Method == "OPTIONS" {
			return
		}
		next := http.HandlerFunc(handler)
		next.ServeHTTP(w, r)
	})
}

func status(w http.ResponseWriter, r *http.Request) {
	status := struct {
		Health string `json:"health"`
	}{
		"healthy",
	}
	j, err := json.Marshal(status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(j)
}

func (s *Server) HandleListRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		httpError(w, "invalid method", http.StatusBadRequest)
		return
	}
	requests, err := s.Storage.Read(storage.BUCKET_API, storage.KEY_REQUESTS)
	if err != nil {
		log.Print("error reading requests: ", err)
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(requests)
	if err != nil {
		log.Print("error encoding response: ", err)
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) HandlePostRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		httpError(w, "invalid method", http.StatusBadRequest)
		return
	}

	var req request.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Song == "" || req.Artist == "" {
		httpError(w, "song and artist required", http.StatusBadRequest)
		return
	}
	req.Time = time.Now()
	if err := s.Storage.CheckPermission(req.Session); err != nil {
		log.Print("error checking permission: ", err)
		httpError(w, err.Error(), http.StatusForbidden)
		return
	}

	requests, err := s.Storage.Read(storage.BUCKET_API, storage.KEY_REQUESTS)
	if err != nil {
		httpError(w, err.Error(), http.StatusForbidden)
		return
	}
	requests = append(requests, req)

	if err := s.Storage.Write(storage.BUCKET_API, storage.KEY_REQUESTS, requests); err != nil {
		log.Print("error writing request: ", err)
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := email.SendEmail(req); err != nil {
		log.Print("error sending email: ", err)
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(req)
	if err != nil {
		log.Print("error encoding response: ", err)
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) HandleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		httpError(w, "invalid method", http.StatusBadRequest)
		return
	}

	var req request.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.Storage.CheckPermission(req.Session); err != nil {
		log.Print("error checking permission: ", err)
		httpError(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := s.SMS.Send(req); err != nil {
		log.Print("error sending sms: ", err)
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println("request received: ", req)
	w.Header().Add("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(req)
	if err != nil {
		log.Print("error encoding response: ", err)
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) HandleListPhotos(w http.ResponseWriter, r *http.Request) {
	reader, err := s.Storage.Get(storage.BUCKET_API, storage.KEY_PHOTOS)
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var metadata map[string]photo.Metadata
	if err = json.NewDecoder(reader).Decode(&metadata); err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	thumbList, err := s.Storage.List(storage.BUCKET_THUMBNAILS)
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	//TODO
	type Photo struct {
		Body     []byte         `json:"body"`
		Metadata photo.Metadata `json:"metadata"`
	}
	var photos []Photo

	// TODO: parallelize or s3 dump all at once
	for _, thumb := range thumbList {
		thumbReader, err := s.Storage.Get(storage.BUCKET_THUMBNAILS, thumb)
		if err != nil {
			httpError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		body := &bytes.Buffer{}
		if _, err := io.Copy(body, thumbReader); err != nil {
			httpError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		photos = append(photos, Photo{
			Body:     body.Bytes(),
			Metadata: metadata[thumb],
		})
	}

	w.Header().Add("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(photos)
	if err != nil {
		log.Print("error encoding response: ", err)
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) HandleUpdatePhotos(w http.ResponseWriter, r *http.Request) {
	var photoMetadata map[string]photo.Metadata
	err := json.NewDecoder(r.Body).Decode(&photoMetadata)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}
	reader, err := s.Storage.Get(storage.BUCKET_API, storage.KEY_PHOTOS)
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var metadata map[string]photo.Metadata
	err = json.NewDecoder(reader).Decode(&metadata)
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for k, v := range photoMetadata {
		metadata[k] = v
	}
	if err = s.Storage.Write(storage.BUCKET_API, storage.KEY_PHOTOS, metadata); err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(photoMetadata)
	if err != nil {
		log.Print("error encoding response: ", err)
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) HandleUploadPhotos(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		httpError(w, "invalid method", http.StatusBadRequest)
		return
	}

	var photoRequests []photo.GooglePhotoRequest
	err := json.NewDecoder(r.Body).Decode(&photoRequests)
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	metadataMap := make(map[string]photo.Metadata)
	for _, photoRequest := range photoRequests {
		file, err := photo.GetGooglePhoto(photoRequest)
		if err != nil {
			httpError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer os.Remove(file.Name())
		if photoRequest.MimeType != "image/jpeg" {
			httpError(w, "invalid mime type", http.StatusBadRequest)
			return
		}
		// TODO enable png if/when we get converter below working
		if !strings.HasSuffix(strings.ToLower(photoRequest.Filename), ".jpg") && !strings.HasSuffix(strings.ToLower(photoRequest.Filename), ".jpeg") {
			httpError(w, "invalid file extension", http.StatusBadRequest)
			return
		}

		if strings.HasSuffix(strings.ToLower(photoRequest.Filename), ".png") {
			err = photo.PngToJpg(file.Name(), file) // TODO not working
			if err != nil {
				httpError(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		// thumbnail
		thumbnail, err := photo.CreateThumbnail(file.Name())
		fmt.Println("A", err)
		if err != nil {
			log.Println("error creating thumbnail: ", err)
		}
		defer os.Remove(thumbnail)
		if err = s.Storage.Upload(storage.BUCKET_THUMBNAILS, photoRequest.ID, thumbnail); err != nil {
			httpError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err = s.Storage.Upload(storage.BUCKET_IMAGES, photoRequest.ID, file.Name()); err != nil {
			httpError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		metadataMap[photoRequest.ID] = photoRequest.Metadata
	}
	// update custom metadata
	res, err := s.Storage.Get(storage.BUCKET_API, storage.KEY_PHOTOS)
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var metadata map[string]photo.Metadata
	err = json.NewDecoder(res).Decode(&metadata)
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	photo.UpdateMetadata(metadataMap, metadata)
	if err = s.Storage.Write(storage.BUCKET_API, storage.KEY_PHOTOS, metadata); err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(photoRequests); err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// HandleProtected is a handler to test protected routes.
func (s *Server) HandleProtected(w http.ResponseWriter, r *http.Request) {
	keys, err := s.Storage.List(storage.BUCKET_IMAGES)
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	f, err := s.Storage.Get(storage.BUCKET_IMAGES, keys[0])
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	metadata, err := photo.GetExifData(f)
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(metadata)
	if err != nil {
		log.Print("error encoding response: ", err)
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func httpError(w http.ResponseWriter, errStr string, code int) {
	j, err := json.Marshal(map[string]interface{}{
		"error": errStr,
		"code":  code,
	})
	if err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.Write(j)
}
