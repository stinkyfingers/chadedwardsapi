package email

import (
	"fmt"
	"net/smtp"
	"os"
	"strings"

	"github.com/stinkyfingers/chadedwardsapi/request"
)

func SendEmail(req request.Request) error {
	auth := smtp.PlainAuth("", os.Getenv("GMAIL_EMAIL"), os.Getenv("GMAIL_PASSWORD"), "smtp.gmail.com")
	to := strings.Split(os.Getenv("GMAIL_DESTINATION"), ",")
	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\nSong: %s\nArtist: %s\nFrom: %s\nMessage: %s", os.Getenv("GMAIL_DESTINATION"), "Song Request", req.Song, req.Artist, req.Name, req.Message))
	err := smtp.SendMail("smtp.gmail.com:587", auth, os.Getenv("GMAIL_EMAIL"), to, msg)
	if err != nil {
		return err
	}
	return nil
}
