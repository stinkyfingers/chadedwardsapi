package server

import (
	"bytes"
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
	"github.com/pkg/errors"
)

type Server struct {
	Storage *s3.S3
}

type Request struct {
	Session string `json:"session"`
	Name    string `json:"name"`
	Message string `json:"message"`
	Song    string `json:"song"`
	Artist  string `json:"artist"`
}

type Suggestion struct {
	Message string `json:"message"`
	Song    string `json:"song"`
	Artist  string `json:"artist"`
}

type SMSRequestBody struct {
	From      string `json:"from"`
	Text      string `json:"text"`
	To        string `json:"to"`
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
}

type SMSResponseBody struct {
	Messages []Message `json:"messages"`
}
type Message struct {
	To               string `json:"to"`
	MessageID        string `json:"message-id"`
	Status           string `json:"status"` // 0 = ok
	ErrorText        string `json:"error-text"`
	RemainingBalance string `json:"remaining-balance"`
	MessagePrice     string `json:"message-price"`
	Network          string `json:"network"`
}

type Permission map[string]time.Time // ip:time

var (
	timeout = time.Minute * 10
)

func NewServer(profile string) (*Server, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: profile,
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
func NewMux(s *Server) (http.Handler, error) {

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
	if err := s.checkPermission(req.Session); err != nil {
		log.Print("error checking permission: ", err)
		httpError(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := sendSMSs(req); err != nil {
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

func (s *Server) checkPermission(session string) error {
	resp, err := s.Storage.GetObject(&s3.GetObjectInput{
		Bucket: aws.String("chadedwardsapi"),
		Key:    aws.String("session-blacklist"),
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
	for session, timestamp := range permissions {
		if session == session && time.Now().Add(-1*timeout).Before(timestamp) {
			return fmt.Errorf("permission denied: you must wait 10 minutes before requesting again")
		}
	}

	permissions[session] = time.Now()
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

func sendSMSs(req Request) error {
	var err error
	destinations := strings.Split(os.Getenv("NEXMO_DESTINATION"), ",")
	for _, destination := range destinations {
		if smsErr := sendSMS(req, destination); smsErr != nil {
			err = errors.Wrap(err, smsErr.Error())
		}
	}
	return err
}

func sendSMS(req Request, destination string) error {
	body := SMSRequestBody{
		APIKey:    os.Getenv("NEXMO_KEY"),
		APISecret: os.Getenv("NEXMO_SECRET"),
		To:        destination,
		From:      os.Getenv("NEXMO_SOURCE"),
		Text:      fmt.Sprintf("%s (%s)\nFrom: %s\nMessage: %s", req.Song, req.Artist, req.Name, req.Message),
	}
	smsBody, err := json.Marshal(body)
	if err != nil {
		return err
	}
	r, err := http.NewRequest("POST", "https://rest.nexmo.com/sms/json", bytes.NewBuffer(smsBody))
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")

	cli := &http.Client{}
	resp, err := cli.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var messageResponse SMSResponseBody
	if err = json.NewDecoder(resp.Body).Decode(&messageResponse); err != nil {
		return err
	}
	log.Print("message response: ", messageResponse)
	if len(messageResponse.Messages) == 0 {
		return fmt.Errorf("no messages returned")
	}
	if messageResponse.Messages[0].Status != "0" {
		return fmt.Errorf("message status: %s", messageResponse.Messages[0].ErrorText)
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
