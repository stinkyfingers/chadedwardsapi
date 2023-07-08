package request

import "time"

type Request struct {
	Time    time.Time `json:"time"`
	Session string    `json:"session"`
	Name    string    `json:"name"`
	Message string    `json:"message"`
	Song    string    `json:"song"`
	Artist  string    `json:"artist"`
}
