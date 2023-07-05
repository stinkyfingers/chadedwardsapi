package sms

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"
)

type Twilio struct {
}

func NewTwilio() SMS {
	return &Twilio{}
}

func (t *Twilio) Send(req Request) error {
	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: os.Getenv("TWILIO_USER"),
		Password: os.Getenv("TWILIO_PASS"),
	})

	params := &openapi.CreateMessageParams{}
	params.SetTo(os.Getenv("TWILIO_DESTINATION"))
	params.SetFrom(os.Getenv("TWILIO_SOURCE"))
	params.SetBody(fmt.Sprintf("%s (%s)\nFrom: %s\nMessage: %s", req.Song, req.Artist, req.Name, req.Message))

	resp, err := client.Api.CreateMessage(params)
	if err != nil {
		return err
	}
	j, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	fmt.Println(string(j))
	return nil
}
