package sms

type SMS interface {
	Send(req Request) error
}

type Request struct {
	Session string `json:"session"`
	Name    string `json:"name"`
	Message string `json:"message"`
	Song    string `json:"song"`
	Artist  string `json:"artist"`
}
