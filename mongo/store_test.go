package mongo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ONSdigital/dis-imf-uploader/models"
	"github.com/ONSdigital/dis-imf-uploader/mongo/mock"
	. "github.com/smartystreets/goconvey/convey"
)

const testUploadID = "test-upload-id"

func TestStoreMock(t *testing.T) {
	Convey("Given a mocked store", t, func() {
		mockStore := &mock.StoreMock{
			CreateUploadFunc: func(ctx context.Context, upload *models.Upload) error {
				upload.ID = testUploadID
				upload.UploadedAt = time.Now()
				upload.Status = models.StatusPending
				return nil
			},
			GetUploadFunc: func(ctx context.Context, id string) (*models.Upload, error) {
				if id == testUploadID {
					return &models.Upload{
						ID:          id,
						FileName:    "test.pdf",
						FileSize:    1024,
						ContentType: "application/pdf",
						Status:      models.StatusPending,
					}, nil
				}
				return nil, errors.New("not found")
			},
			UpdateUploadApprovedFunc: func(ctx context.Context, id string, reviewedBy, s3Key, backupKey, invID string) error {
				if id == testUploadID {
					return nil
				}
				return errors.New("not found")
			},
			UpdateUploadRejectedFunc: func(ctx context.Context, id string, reviewedBy, reason string) error {
				if id == testUploadID {
					return nil
				}
				return errors.New("not found")
			},
			LogAuditFunc: func(ctx context.Context, log *models.AuditLog) error {
				log.ID = "test-audit-id"
				return nil
			},
			ListUploadsFunc: func(ctx context.Context, status string, page, pageSize int, sortBy, sortDir string) ([]*models.Upload, int64, error) {
				uploads := []*models.Upload{
					{
						ID:       "upload-1",
						FileName: "file1.pdf",
						Status:   models.StatusPending,
					},
					{
						ID:       "upload-2",
						FileName: "file2.pdf",
						Status:   models.StatusApproved,
					},
				}

				if status != "" {
					filtered := []*models.Upload{}
					for _, u := range uploads {
						if string(u.Status) == status {
							filtered = append(filtered, u)
						}
					}
					return filtered, int64(len(filtered)), nil
				}

				return uploads, int64(len(uploads)), nil
			},
			HealthCheckFunc: func(ctx context.Context) error {
				return nil
			},
			CloseFunc: func() error {
				return nil
			},
		}

		Convey("When creating an upload", func() {
			upload := &models.Upload{
				FileName:     "test.pdf",
				FileSize:     1024,
				ContentType:  "application/pdf",
				UploadedBy:   "user@example.com",
				FileChecksum: "abc123",
			}

			err := mockStore.CreateUpload(context.Background(), upload)

			Convey("Then the upload should be created successfully", func() {
				So(err, ShouldBeNil)
				So(upload.ID, ShouldEqual, testUploadID)
				So(upload.Status, ShouldEqual, models.StatusPending)
				So(len(mockStore.CreateUploadCalls()), ShouldEqual, 1)
			})
		})

		Convey("When getting an upload", func() {
			Convey("With valid ID", func() {
				upload, err := mockStore.GetUpload(context.Background(), testUploadID)

				Convey("Then the upload should be retrieved", func() {
					So(err, ShouldBeNil)
					So(upload, ShouldNotBeNil)
					So(upload.ID, ShouldEqual, testUploadID)
					So(upload.FileName, ShouldEqual, "test.pdf")
				})
			})

			Convey("With invalid ID", func() {
				upload, err := mockStore.GetUpload(context.Background(), "invalid-id")

				Convey("Then an error should be returned", func() {
					So(err, ShouldNotBeNil)
					So(upload, ShouldBeNil)
				})
			})
		})

		Convey("When approving an upload", func() {
			err := mockStore.UpdateUploadApproved(
				context.Background(),
				testUploadID,
				"reviewer@example.com",
				"s3/key",
				"backup/key",
				"inv-123",
			)

			Convey("Then the approval should succeed", func() {
				So(err, ShouldBeNil)
				So(len(mockStore.UpdateUploadApprovedCalls()), ShouldEqual, 1)
			})
		})

		Convey("When rejecting an upload", func() {
			err := mockStore.UpdateUploadRejected(
				context.Background(),
				testUploadID,
				"reviewer@example.com",
				"not acceptable",
			)

			Convey("Then the rejection should succeed", func() {
				So(err, ShouldBeNil)
				So(len(mockStore.UpdateUploadRejectedCalls()), ShouldEqual, 1)
			})
		})

		Convey("When logging audit", func() {
			auditLog := &models.AuditLog{
				UploadID:  testUploadID,
				Action:    "upload",
				UserID:    "user123",
				Timestamp: time.Now(),
			}

			err := mockStore.LogAudit(context.Background(), auditLog)

			Convey("Then the audit log should be created", func() {
				So(err, ShouldBeNil)
				So(auditLog.ID, ShouldEqual, "test-audit-id")
				So(len(mockStore.LogAuditCalls()), ShouldEqual, 1)
			})
		})

		Convey("When listing uploads", func() {
			Convey("Without filter", func() {
				uploads, total, err := mockStore.ListUploads(
					context.Background(),
					"",
					1,
					10,
					"",
					"",
				)

				Convey("Then all uploads should be returned", func() {
					So(err, ShouldBeNil)
					So(uploads, ShouldHaveLength, 2)
					So(total, ShouldEqual, 2)
				})
			})

			Convey("With status filter", func() {
				uploads, total, err := mockStore.ListUploads(
					context.Background(),
					"pending",
					1,
					10,
					"",
					"",
				)

				Convey("Then filtered uploads should be returned", func() {
					So(err, ShouldBeNil)
					So(uploads, ShouldHaveLength, 1)
					So(total, ShouldEqual, 1)
					So(string(uploads[0].Status), ShouldEqual, "pending")
				})
			})
		})

		Convey("When performing health check", func() {
			err := mockStore.HealthCheck(context.Background())

			Convey("Then health check should pass", func() {
				So(err, ShouldBeNil)
				So(len(mockStore.HealthCheckCalls()), ShouldEqual, 1)
			})
		})

		Convey("When closing the store", func() {
			err := mockStore.Close()

			Convey("Then close should succeed", func() {
				So(err, ShouldBeNil)
				So(len(mockStore.CloseCalls()), ShouldEqual, 1)
			})
		})
	})
}
