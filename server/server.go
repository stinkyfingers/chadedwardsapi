package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

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

	mux := http.NewServeMux()
	mux.Handle("/requests", cors(s.HandleListRequests))
	mux.Handle("/request", cors(s.HandlePostRequest))
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

func cors(handler func(w http.ResponseWriter, r *http.Request)) http.Handler {
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
	requests, err := s.Storage.Read()
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

	var req storage.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Song == "" || req.Artist == "" {
		httpError(w, "song and artist required", http.StatusBadRequest)
		return
	}
	req.Time = time.Now()
	//if err := s.Storage.CheckPermission(req.Session); err != nil {
	//	log.Print("error checking permission: ", err)
	//	httpError(w, err.Error(), http.StatusForbidden)
	//	return
	//}

	if err := s.Storage.Write(req); err != nil {
		log.Print("error writing request: ", err)
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(req)
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

	var req sms.Request
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

//func (s *Server) checkPermission(session string) error {
//	resp, err := s.Storage.GetObject(&s3.GetObjectInput{
//		Bucket: aws.String("chadedwardsapi"),
//		Key:    aws.String("session-blacklist"),
//	})
//	if err != nil { // return error unless file doesn't exist
//		if aerr, ok := err.(awserr.Error); ok {
//			if aerr.Code() != s3.ErrCodeNoSuchKey {
//				return err
//			}
//		}
//	}
//
//	permissions := make(Permission)
//	if resp != nil && resp.Body != nil {
//		defer resp.Body.Close()
//		if err = json.NewDecoder(resp.Body).Decode(&permissions); err != nil {
//			return err
//		}
//	}
//	for session, timestamp := range permissions {
//		if session == session && time.Now().Add(-1*timeout).Before(timestamp) {
//			return fmt.Errorf("permission denied: you must wait 10 minutes before requesting again")
//		}
//	}
//
//	permissions[session] = time.Now()
//	j, err := json.Marshal(permissions)
//	if err != nil {
//		return err
//	}
//	_, err = s.Storage.PutObject(&s3.PutObjectInput{
//		Bucket: aws.String("chadedwardsapi"),
//		Key:    aws.String("ip-blacklist"),
//		Body:   aws.ReadSeekCloser(strings.NewReader(string(j))),
//	})
//	if err != nil {
//		return err
//	}
//	return nil
//}

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
