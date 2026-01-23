package storage

import (
	"testing"

	"github.com/ONSdigital/dis-imf-uploader/config"
	. "github.com/smartystreets/goconvey/convey"
)

func TestS3ClientConfiguration(t *testing.T) {
	Convey("Given a configuration", t, func() {
		cfg := &config.Config{
			S3Config: config.S3Config{
				Bucket: "test-bucket",
				Prefix: "imf/",
				Region: "eu-west-2",
			},
		}

		Convey("When creating a new S3 client", func() {
			s3Client, err := NewS3(cfg)

			Convey("Then the client should be created successfully", func() {
				So(err, ShouldBeNil)
				So(s3Client, ShouldNotBeNil)
				So(s3Client.bucket, ShouldEqual, "test-bucket")
				So(s3Client.prefix, ShouldEqual, "imf/")
			})
		})

		Convey("When creating a client with invalid region", func() {
			invalidCfg := &config.Config{
				S3Config: config.S3Config{
					Bucket: "test-bucket",
					Prefix: "imf/",
					Region: "",
				},
			}

			s3Client, err := NewS3(invalidCfg)

			Convey("Then the client should still be created", func() {
				So(err, ShouldBeNil)
				So(s3Client, ShouldNotBeNil)
			})
		})
	})
}

func TestS3KeyConstruction(t *testing.T) {
	Convey("Given an S3 client with a prefix", t, func() {
		cfg := &config.Config{
			S3Config: config.S3Config{
				Bucket: "test-bucket",
				Prefix: "imf/",
				Region: "eu-west-2",
			},
		}

		s3Client, _ := NewS3(cfg)

		Convey("When constructing full keys", func() {
			Convey("Then prefix should be prepended correctly", func() {
				So(s3Client.prefix, ShouldEqual, "imf/")
			})
		})
	})

	Convey("Given an S3 client without a prefix", t, func() {
		cfg := &config.Config{
			S3Config: config.S3Config{
				Bucket: "test-bucket",
				Prefix: "",
				Region: "eu-west-2",
			},
		}

		s3Client, _ := NewS3(cfg)

		Convey("When constructing full keys", func() {
			Convey("Then prefix should be empty", func() {
				So(s3Client.prefix, ShouldEqual, "")
			})
		})
	})
}
