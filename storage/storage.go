package storage

import (
	"io"
	"time"
)

type Storage interface {
	Write(bucket, key string, o obj) error
	Read(bucket, key string) ([]obj, error)
	Get(bucket, key string) (io.ReadCloser, error)
	List(bucket string) ([]string, error)
	Delete(bucket, key string) error
	Upload(bucket, key, filename string) error
	CheckPermission(session string) error
}

type obj interface{}

type Permission map[string]time.Time // ip:time
var (
	timeout = time.Minute * 10
)
