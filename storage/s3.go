package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/ONSdigital/dis-imf-uploader/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const errNotFound = "NotFound"

// S3Client provides methods for interacting with AWS S3 storage.
type S3Client struct {
	client *s3.Client
	bucket string
	prefix string
}

// NewS3 creates a new S3 client with the provided configuration.
func NewS3(cfg *config.Config) (*S3Client, error) {
	awsCfg, err := awsConfig.LoadDefaultConfig(
		context.Background(),
		awsConfig.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, err
	}

	return &S3Client{
		client: s3.NewFromConfig(awsCfg),
		bucket: cfg.Bucket,
		prefix: cfg.S3Config.Prefix,
	}, nil
}

// CheckFileExists checks if a file exists in S3 at the given key.
func (s *S3Client) CheckFileExists(
	ctx context.Context,
	key string,
) (bool, error) {
	fullKey := s.prefix + key

	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fullKey),
	})

	if err == nil {
		return true, nil
	}

	// Check for NotFound error (404)
	if err.Error() == errNotFound ||
		(err != nil && (err.Error() == errNotFound ||
			err.Error() == "404")) {
		return false, nil
	}

	return false, err
}

// BackupFile creates a backup copy of a file in S3 and returns the backup key.
func (s *S3Client) BackupFile(
	ctx context.Context,
	fileName string,
) (string, error) {
	source := s.prefix + fileName
	backupKey := fmt.Sprintf("backup/%d/%s", time.Now().Unix(), fileName)
	backupPath := s.prefix + backupKey

	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucket),
		CopySource: aws.String(s.bucket + "/" + source),
		Key:        aws.String(backupPath),
	})

	if err != nil {
		return "", err
	}

	return backupKey, nil
}

// UploadFile uploads a file to S3 at the specified key.
func (s *S3Client) UploadFile(ctx context.Context, key string, body io.Reader) error {
	fullKey := s.prefix + key

	// Read body into bytes to allow retry
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fullKey),
		Body:   bytes.NewReader(bodyBytes),
	})

	return err
}

// DeleteFile deletes a file from S3 at the specified key.
func (s *S3Client) DeleteFile(ctx context.Context, key string) error {
	fullKey := s.prefix + key

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(fullKey),
	})

	return err
}
