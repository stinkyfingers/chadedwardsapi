package sms

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/pkg/errors"
)

type Nexmo struct{}

type NexmoRequestBody struct {
	From      string `json:"from"`
	Text      string `json:"text"`
	To        string `json:"to"`
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
}

type NexmoResponseBody struct {
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

func NewNexmo() SMS {
	return &Nexmo{}
}

func (n *Nexmo) Send(req Request) error {
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
	body := NexmoRequestBody{
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
	var messageResponse NexmoResponseBody
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
