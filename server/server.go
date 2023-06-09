package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"
)

type Server struct {
}

func NewServer() *Server {
	return &Server{}
}

// NewMux returns the router
func NewMux() (http.Handler, error) {
	s := NewServer()
	mux := http.NewServeMux()
	mux.Handle("/sms", cors(s.HandleSendSMS))
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

func (s *Server) HandleSendSMS(w http.ResponseWriter, r *http.Request) {
	sid := os.Getenv("TWILIO_USER")
	token := os.Getenv("TWILIO_PASS")
	if sid == "" || token == "" {
		http.Error(w, "missing twilio credentials", http.StatusInternalServerError)
		return
	}
	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: sid,
		Password: token,
	})

	params := &openapi.CreateMessageParams{}
	params.SetTo(os.Getenv("TWILIO_DESTINATION")) //TODO, split
	params.SetFrom(os.Getenv("TWILIO_SOURCE"))
	params.SetBody("Hello from Golang!") // TODO

	_, err := client.Api.CreateMessage(params)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("SMS sent successfully!")
	}
}
