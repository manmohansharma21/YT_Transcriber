package s3

import (
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
)

type BucketOperator interface {
	Push(File) error
	Pull(data MetaData) (File, error)
	// Add the GetService method to return the svc field
	GetService() *session.Session
}

type File struct {
	MetaData
	Content io.Reader
}

func (m MetaData) S3Path() string {
	return fmt.Sprintf("%s", m.Filename)
}

func (f File) ContentBytes() ([]byte, error) {
	return io.ReadAll(f.Content)
}

type MetaData struct {
	Filename        string
	Source          string
	CreatedTime     time.Time
	CreatedBy       string
	LastUpdatedDate time.Time
	LastEditedBy    string
}
