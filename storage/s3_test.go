package storage

import (
	"bytes"
	"context"
	"testing"

	"github.com/ONSdigital/dis-imf-uploader/config"
	. "github.com/smartystreets/goconvey/convey"
)

func TestS3Client(t *testing.T) {
	Convey("Given an S3 client", t, func() {
		// Note: Use mocking or local S3 (minio) for tests
		cfg := &config.Config{
			S3Config: config.S3Config{
				Bucket:      "test-bucket",
				Prefix:      "imf/",
				Region:      "eu-west-2",
				EndpointURL: "http://localhost:9000", // minio
			},
		}

		s3Client, err := NewS3(cfg)
		So(err, ShouldBeNil)

		Convey("When uploading a file", func() {
			ctx := context.Background()
			fileContent := []byte("test file content")
			body := bytes.NewReader(fileContent)

			err := s3Client.UploadFile(ctx, "test.txt", body)

			Convey("Then the file should be uploaded successfully", func() {
				So(err, ShouldBeNil)
			})

			Convey("And the file should exist", func() {
				exists, err := s3Client.CheckFileExists(ctx, "test.txt")
				So(err, ShouldBeNil)
				So(exists, ShouldBeTrue)
			})
		})

		Convey("When checking a non-existent file", func() {
			exists, err := s3Client.CheckFileExists(context.Background(), "nonexistent.txt")

			Convey("Then it should return false", func() {
				So(err, ShouldBeNil)
				So(exists, ShouldBeFalse)
			})
		})

		Convey("When backing up a file", func() {
			ctx := context.Background()
			s3Client.UploadFile(ctx, "original.pdf", bytes.NewReader([]byte("content")))

			backupKey, err := s3Client.BackupFile(ctx, "original.pdf")

			Convey("Then backup should be created", func() {
				So(err, ShouldBeNil)
				So(backupKey, ShouldNotBeBlank)
				So(backupKey, ShouldContainSubstring, "backup/")
			})

			Convey("And backup file should exist", func() {
				exists, _ := s3Client.CheckFileExists(ctx, backupKey)
				So(exists, ShouldBeTrue)
			})
		})
	})
}
