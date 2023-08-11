package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	BUCKET_API        = "chadedwardsapi"
	BUCKET_IMAGES     = "chadedwardsbandimages"
	BUCKET_THUMBNAILS = "chadedwardsbandthumbnails"
	blacklistKey      = "session-blacklist"
	KEY_REQUESTS      = "requests"
	KEY_PHOTOS        = "photos.json"
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

func (s *S3) Write(bucket, key string, object obj) error {
	j, err := json.Marshal(object)
	if err != nil {
		return err
	}
	_, err = s.Session.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   aws.ReadSeekCloser(strings.NewReader(string(j))),
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *S3) Read(bucket, key string) ([]obj, error) {
	resp, err := s.Session.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil { // return error unless file doesn't exist
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == s3.ErrCodeNoSuchKey {
				return []obj{}, nil
			}
		}
		return nil, err
	}
	var objects []obj
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
		if err = json.NewDecoder(resp.Body).Decode(&objects); err != nil {
			return nil, err
		}
	}
	return objects, nil
}

func (s *S3) Get(bucket, key string) (io.ReadCloser, error) {
	resp, err := s.Session.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == s3.ErrCodeNoSuchKey {
				return io.NopCloser(&bytes.Buffer{}), nil
			}
		}
		return nil, err
	}
	return resp.Body, nil
}

func (s *S3) List(bucket string) ([]string, error) {
	var keys []string
	err := s.Session.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, o := range page.Contents {
			keys = append(keys, *o.Key)
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *S3) Upload(bucket, key, filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	buffer := make([]byte, info.Size())
	f.Read(buffer)
	fileType := http.DetectContentType(buffer)
	_, err = s.Session.PutObject(&s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(buffer),
		ContentLength: aws.Int64(info.Size()),
		ContentType:   aws.String(fileType), // e.g. image/jpeg
	})
	return err
}

func (s *S3) Delete(bucket, key string) error {
	_, err := s.Session.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *S3) CheckPermission(session string) error {
	resp, err := s.Session.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(BUCKET_API),
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
		Bucket: aws.String(BUCKET_API),
		Key:    aws.String(blacklistKey),
		Body:   aws.ReadSeekCloser(strings.NewReader(string(j))),
	})
	if err != nil {
		return err
	}
	return nil
}
