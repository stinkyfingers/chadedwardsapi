package storage

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type S3 struct {
	Session *s3.S3
}

const (
	bucket       = "chadedwardsapi"
	blacklistKey = "session-blacklist"
	requestsKey  = "requests"
)

func NewS3(profile string) (*S3, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: profile,
		Config: aws.Config{
			Region: aws.String("us-west-1"),
		},
	})
	if err != nil {
		return nil, err
	}

	return &S3{
		Session: s3.New(sess),
	}, nil
}

func (s *S3) Write(req Request) error {
	requests, err := s.Read()
	if err != nil {
		return err
	}
	requests = append(requests, req)
	j, err := json.Marshal(requests)
	if err != nil {
		return err
	}
	_, err = s.Session.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(requestsKey),
		Body:   aws.ReadSeekCloser(strings.NewReader(string(j))),
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *S3) Read() ([]Request, error) {
	resp, err := s.Session.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(requestsKey),
	})
	if err != nil { // return error unless file doesn't exist
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() != s3.ErrCodeNoSuchKey {
				return nil, err
			}
		}
		return []Request{}, nil
	}
	var requests []Request
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
		if err = json.NewDecoder(resp.Body).Decode(&requests); err != nil {
			return nil, err
		}
	}
	return requests, nil
}

func (s *S3) CheckPermission(session string) error {
	resp, err := s.Session.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(blacklistKey),
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
	_, err = s.Session.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(blacklistKey),
		Body:   aws.ReadSeekCloser(strings.NewReader(string(j))),
	})
	if err != nil {
		return err
	}
	return nil
}
