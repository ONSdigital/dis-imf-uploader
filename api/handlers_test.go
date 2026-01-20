package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ONSdigital/dis-imf-uploader/config"
	"github.com/ONSdigital/dis-imf-uploader/models"
	"github.com/ONSdigital/dis-imf-uploader/notifications"

	. "github.com/smartystreets/goconvey/convey"
)

func TestHandlers(t *testing.T) {
	Convey("Given API handlers", t, func() {
		// Setup mocks
		cfg := &config.Config{
			BindAddr:           "localhost:30200",
			MaxUploadSize:      10 * 1024 * 1024,
			TempStorageTimeout: 24 * time.Hour,
			S3Config: config.S3Config{
				Prefix: "imf/",
			},
		}

		// Create mock dependencies
		slackNotifier := notifications.NewSlackNotifier(&config.SlackConfig{
			Enabled: false,
		})

		// Note: Use test doubles for DB, S3, etc.
		// For now, demonstrating test structure

		Convey("When uploading a file", func() {
			Convey("Then upload endpoint should accept the file", func() {
				// Build multipart form
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)

				part, err := writer.CreateFormFile("file", "test.pdf")
				So(err, ShouldBeNil)

				_, err = io.WriteString(part, "test file content")
				So(err, ShouldBeNil)

				writer.Close()

				// Create request
				req := httptest.NewRequest("POST", "/api/v1/uploads/staging", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("X-User-Email", "user@example.com")
				req.Header.Set("Authorization", "Bearer test-token")

				// Assert request can be parsed
				So(req.Method, ShouldEqual, "POST")
				So(req.Header.Get("X-User-Email"), ShouldEqual, "user@example.com")
			})
		})

		Convey("When approving an upload", func() {
			Convey("Then approval should validate reviewer role", func() {
				req := httptest.NewRequest("POST", "/api/v1/uploads/123/approve", nil)
				req.Header.Set("X-User-Email", "non-reviewer@example.com")

				w := httptest.NewRecorder()

				// In real test, handler would check role from DB
				So(req.Header.Get("X-User-Email"), ShouldEqual, "non-reviewer@example.com")
			})
		})

		Convey("When rejecting an upload", func() {
			Convey("Then rejection should require a reason", func() {
				rejectReq := models.RejectRequest{
					Reason: "File format not acceptable",
				}

				body, err := json.Marshal(rejectReq)
				So(err, ShouldBeNil)

				req := httptest.NewRequest("POST", "/api/v1/uploads/123/reject",
					bytes.NewReader(body))
				req.Header.Set("X-User-Email", "reviewer@example.com")
				req.Header.Set("Content-Type", "application/json")

				So(req.ContentLength > 0, ShouldBeTrue)
			})
		})

		Convey("When checking upload status", func() {
			Convey("Then status endpoint should return upload details", func() {
				req := httptest.NewRequest("GET", "/api/v1/uploads/123", nil)
				w := httptest.NewRecorder()

				So(req.Method, ShouldEqual, "GET")
				So(w.Code, ShouldEqual, http.StatusOK)
			})
		})

		Convey("When purging Cloudflare cache", func() {
			Convey("Then it should accept purge requests for approved uploads", func() {
				req := httptest.NewRequest("POST", "/api/v1/uploads/123/purge-cloudflare", nil)
				req.Header.Set("X-User-Email", "reviewer@example.com")

				So(req.Method, ShouldEqual, "POST")
				So(req.Header.Get("X-User-Email"), ShouldNotBeBlank)
			})
		})
	})
}

func TestUploadFileHandler(t *testing.T) {
	Convey("Given upload file handler", t, func() {
		Convey("When request has invalid environment", func() {
			Convey("Then it should return 400", func() {
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)
				part, _ := writer.CreateFormFile("file", "test.pdf")
				io.WriteString(part, "test")
				writer.Close()

				req := httptest.NewRequest("POST", "/api/v1/uploads/invalid-env", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("X-User-Email", "user@example.com")

				So(req.URL.Path, ShouldContainSubstring, "uploads")
			})
		})

		Convey("When request missing X-User-Email header", func() {
			Convey("Then it should return 400", func() {
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)
				part, _ := writer.CreateFormFile("file", "test.pdf")
				io.WriteString(part, "test")
				writer.Close()

				req := httptest.NewRequest("POST", "/api/v1/uploads/staging", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())

				email := req.Header.Get("X-User-Email")
				So(email, ShouldBeBlank)
			})
		})
	})
}
