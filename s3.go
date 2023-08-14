package main

import (
	"bytes"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type Options struct {
	Region string
	Bucket string
}

func New(options Options) (BucketOperator, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(options.Region)},
	)
	if err != nil {
		return nil, err
	}
	return &S3{
		s3Uploader:   s3manager.NewUploader(sess),
		s3Downloader: s3manager.NewDownloader(sess),
		svc:          s3.New(sess),
		bucket:       options.Bucket,
		session:      *sess,
	}, nil
}

func (s *S3) GetService() *session.Session {
	return &s.session
}

type S3 struct {
	s3Uploader   *s3manager.Uploader
	s3Downloader *s3manager.Downloader
	svc          *s3.S3
	bucket       string
	session      session.Session
}

func (s *S3) Push(file File) error {
	data, err := file.ContentBytes()
	if err != nil {
		return fmt.Errorf("failed to read file contents: %v", err)
	}
	_, err = s.s3Uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(file.S3Path()),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file to s3: %s", err.Error())
	}
	return nil
}

func (s *S3) Pull(data MetaData) (File, error) {
	buf := aws.NewWriteAtBuffer([]byte{})
	_, err := s.s3Downloader.Download(buf,
		&s3.GetObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(data.S3Path()),
		})
	return File{
		MetaData: data,
		Content:  bytes.NewBuffer(buf.Bytes()),
	}, err
}
