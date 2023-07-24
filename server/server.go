package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/stinkyfingers/chadedwardsapi/auth"
	"github.com/stinkyfingers/chadedwardsapi/email"
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

	mux := http.NewServeMux()
	mux.Handle("/requests", cors(s.HandleListRequests))
	mux.Handle("/request", cors(s.HandlePostRequest))
	mux.Handle("/login", cors(s.HandleLogin))
	mux.Handle("/auth", cors(authMiddleware(s.HandleProtected))) // route to test auth
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

func authMiddleware(handler func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if err := auth.VerifyJWT(token); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		next := http.HandlerFunc(handler)
		next.ServeHTTP(w, r)
	}
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

	if err := s.Storage.Write(req); err != nil {
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

func (s *Server) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		httpError(w, "invalid method", http.StatusBadRequest)
		return
	}

	user := r.URL.Query().Get("username")
	pass := r.URL.Query().Get("password")
	signedToken, err := auth.JWT()
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	token, err := s.Storage.Login(auth.Authentication{Username: user, Password: pass, SignedToken: signedToken})
	if err != nil {
		httpError(w, err.Error(), http.StatusUnauthorized)
		return
	}
	w.Header().Add("Content-Type", "application/text")
	err = json.NewEncoder(w).Encode(token)
	if err != nil {
		log.Print("error encoding response: ", err)
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) HandleProtected(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("protected"))
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
