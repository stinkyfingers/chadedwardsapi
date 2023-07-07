package storage

import "time"

type Storage interface {
	Write(r Request) error
	Read() ([]Request, error)
	CheckPermission(session string) error
}

type Request struct {
	Time    time.Time `json:"time"`
	Session string    `json:"session"`
	Name    string    `json:"name"`
	Message string    `json:"message"`
	Song    string    `json:"song"`
	Artist  string    `json:"artist"`
}

type Permission map[string]time.Time // ip:time
var (
	timeout = time.Minute * 10
)
