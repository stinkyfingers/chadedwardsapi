package sms

import "github.com/stinkyfingers/chadedwardsapi/request"

type SMS interface {
	Send(req request.Request) error
}
