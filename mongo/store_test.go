package mongo

import (
	"context"
	"testing"

	"github.com/ONSdigital/dis-imf-uploader/config"
	"github.com/ONSdigital/dis-imf-uploader/models"
	dpMongo "github.com/ONSdigital/dp-mongodb/v3/mongodb"
	. "github.com/smartystreets/goconvey/convey"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestDatabase(t *testing.T) {
	Convey("Given a database connection", t, func() {
		// Setup: Create test database
		cfg := config.MongoConfig{
			MongoDriverConfig: dpMongo.MongoDriverConfig{
				ClusterEndpoint: "localhost:27017",
				Database:        "test_file_uploads",
			},
		}

		db, err := New(cfg)
		So(err, ShouldBeNil)
		defer db.Close()

		Convey("When creating an upload", func() {
			upload := &models.Upload{
				FileName:     "test.pdf",
				FileSize:     1024,
				ContentType:  "application/pdf",
				UploadedBy:   "user@example.com",
				FileChecksum: "abc123",
			}

			err := db.CreateUpload(context.Background(), upload)

			Convey("Then the upload should be created successfully", func() {
				So(err, ShouldBeNil)
				So(upload.ID, ShouldNotEqual, primitive.NilObjectID)
				So(upload.Status, ShouldEqual, models.StatusPending)
			})

			Convey("And the upload should be retrievable", func() {
				retrieved, err := db.GetUpload(context.Background(), upload.ID)
				So(err, ShouldBeNil)
				So(retrieved, ShouldNotBeNil)
				So(retrieved.FileName, ShouldEqual, "test.pdf")
			})
		})

		Convey("When updating an upload to approved", func() {
			upload := &models.Upload{
				FileName:     "doc.pdf",
				FileSize:     2048,
				ContentType:  "application/pdf",
				UploadedBy:   "user@example.com",
				FileChecksum: "def456",
			}

			db.CreateUpload(context.Background(), upload)
			err := db.UpdateUploadApproved(context.Background(), upload.ID,
				"reviewer@example.com", "imf/doc.pdf", "backup/doc.pdf", "inv-123")

			Convey("Then the status should be approved", func() {
				So(err, ShouldBeNil)
				retrieved, _ := db.GetUpload(context.Background(), upload.ID)
				So(retrieved.Status, ShouldEqual, models.StatusApproved)
				So(retrieved.ReviewedBy, ShouldEqual, "reviewer@example.com")
				So(retrieved.CloudFrontInvID, ShouldEqual, "inv-123")
			})
		})

		Convey("When creating a user", func() {
			user := &models.User{
				Email: "reviewer@example.com",
				Name:  "John Reviewer",
				Role:  models.RoleReviewer,
			}

			err := db.CreateUser(context.Background(), user)

			Convey("Then the user should be created", func() {
				So(err, ShouldBeNil)
				So(user.ID, ShouldNotEqual, primitive.NilObjectID)
			})

			Convey("And the user should be retrievable by email", func() {
				retrieved, err := db.GetUserByEmail(context.Background(), "reviewer@example.com")
				So(err, ShouldBeNil)
				So(retrieved, ShouldNotBeNil)
				So(retrieved.Role, ShouldEqual, models.RoleReviewer)
			})
		})
	})
}
