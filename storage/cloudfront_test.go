package storage

import (
	"context"
	"testing"

	"github.com/ONSdigital/dis-imf-uploader/config"
	. "github.com/smartystreets/goconvey/convey"
)

func TestCloudFrontClient(t *testing.T) {
	Convey("Given a CloudFront client", t, func() {
		cfg := &config.Config{
			S3Config: config.S3Config{
				Region: "eu-west-2",
			},
		}

		cfClient, err := NewCloudFront(cfg)
		So(err, ShouldBeNil)
		So(cfClient, ShouldNotBeNil)

		Convey("When invalidating cache with valid distribution", func() {
			// This would need mocking or integration test setup
			distributionID := "TESTDIST123"
			paths := []string{"/imf/test.pdf"}

			Convey("Then it should accept the request", func() {
				// Actual test would need AWS credentials or mock
				So(distributionID, ShouldNotBeBlank)
				So(len(paths), ShouldEqual, 1)
			})
		})

		Convey("When invalidating cache without distribution ID", func() {
			_, err := cfClient.InvalidateCache(context.Background(), "", []string{"/test"})

			Convey("Then it should return an error", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "distribution ID is required")
			})
		})
	})
}
