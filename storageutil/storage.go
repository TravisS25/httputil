package storageutil

import (
	"io"
	"net/url"
	"time"

	minio "github.com/minio/minio-go"
)

type StorageReaderWriter interface {
	GetObject(bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error)
	PresignedGetObject(bucketName, objectName string, expiry time.Duration, reqParams url.Values) (*url.URL, error)
	PutObject(bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (n int64, err error)
	RemoveObject(bucketName, objectName string) error
}
