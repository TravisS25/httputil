package storagetest

import (
	"io"
	"net/url"
	"time"

	minio "github.com/minio/minio-go"
)

type MockStorageReaderWriter struct {
	GetObjectFunc          func(bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error)
	PresignedGetObjectFunc func(bucketName, objectName string, expiry time.Duration, reqParams url.Values) (*url.URL, error)
	PutObjectFunc          func(bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (n int64, err error)
	RemoveObjectFunc       func(bucketName, objectName string) error
}

func (m *MockStorageReaderWriter) GetObject(bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	return m.GetObjectFunc(bucketName, objectName, opts)
}

func (m *MockStorageReaderWriter) PresignedGetObject(bucketName, objectName string, expiry time.Duration, reqParams url.Values) (*url.URL, error) {
	return m.PresignedGetObjectFunc(bucketName, objectName, expiry, reqParams)
}

func (m *MockStorageReaderWriter) PutObject(bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (n int64, err error) {
	return m.PutObjectFunc(bucketName, objectName, reader, objectSize, opts)
}

func (m *MockStorageReaderWriter) RemoveObject(bucketName, objectName string) error {
	return m.RemoveObjectFunc(bucketName, objectName)
}
