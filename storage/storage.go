package storage

import (
	"time"

	"github.com/stinkyfingers/chadedwardsapi/auth"
	"github.com/stinkyfingers/chadedwardsapi/request"
)

type Storage interface {
	Write(r request.Request) error
	Read() ([]request.Request, error)
	CheckPermission(session string) error
	Login(authentication auth.Authentication) (string, error)
}

type Permission map[string]time.Time // ip:time
var (
	timeout = time.Minute * 10
)
