package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/twilio/twilio-go"
	"github.com/twilio/twilio-go/rest/api/v2010"
)

type Server struct {
	Storage *s3.S3
}

type Request struct {
	IP      string `json:"ip"`
	Message string `json:"message"`
	Song    string `json:"song"`
	Artist  string `json:"artist"`
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

func NewServer() (*Server, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: "jds", // TODO used locally only
		Config: aws.Config{
			Region: aws.String("us-west-1"),
		},
	})

	if err != nil {
		return nil, err
	}
	return &Server{
		Storage: s3.New(sess),
	}, nil
}

// NewMux returns the router
func NewMux() (http.Handler, error) {
	s, err := NewServer()
	if err != nil {
		log.Fatalln(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/request", cors(s.HandleRequest))
	mux.Handle("/health", cors(status))
	return mux, nil
}

func isPermittedOrigin(origin string) string {
	var permittedOrigins = []string{
		"http://localhost:3000",
		"https://chadedwardsband.com",
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
		log.Println("origin", r.Header.Get("Origin"), r.Header)
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

func (s *Server) HandleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		httpError(w, "invalid method", http.StatusBadRequest)
		return
	}

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.checkPermission(req.IP); err != nil {
		log.Print(err)
		httpError(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := sendSMS(req); err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(req)
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) checkPermission(ip string) error {
	resp, err := s.Storage.GetObject(&s3.GetObjectInput{
		Bucket: aws.String("chadedwardsapi"),
		Key:    aws.String("ip-blacklist"),
	})
	if err != nil { // return error unless file doesn't exist
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() != s3.ErrCodeNoSuchKey {
				return err
			}
		}
	}

	permissions := make(Permission)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
		if err = json.NewDecoder(resp.Body).Decode(&permissions); err != nil {
			return err
		}
	}
	for ipAddress, timestamp := range permissions {
		if ipAddress == ip && time.Now().Add(-1*timeout).Before(timestamp) {
			return fmt.Errorf("permission denied: you must wait 10 minutes before requesting again")
		}
	}

	permissions[ip] = time.Now()
	j, err := json.Marshal(permissions)
	if err != nil {
		return err
	}
	_, err = s.Storage.PutObject(&s3.PutObjectInput{
		Bucket: aws.String("chadedwardsapi"),
		Key:    aws.String("ip-blacklist"),
		Body:   aws.ReadSeekCloser(strings.NewReader(string(j))),
	})
	if err != nil {
		return err
	}
	return nil
}

func sendSMS(req Request) error {
	sid := os.Getenv("TWILIO_USER")
	token := os.Getenv("TWILIO_PASS")
	if sid == "" || token == "" {
		return fmt.Errorf("missing twilio credentials")
	}
	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: sid,
		Password: token,
	})
	destinations := strings.Split(os.Getenv("TWILIO_DESTINATION"), ",")
	msg := fmt.Sprintf("%s - %s\n%s", req.Song, req.Artist, req.Message)
	for _, destination := range destinations {
		params := &openapi.CreateMessageParams{}
		params.SetTo(destination)
		params.SetFrom(os.Getenv("TWILIO_SOURCE"))
		params.SetBody(msg)

		_, err := client.Api.CreateMessage(params)
		if err != nil {
			return err
		}
	}
	return nil
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
