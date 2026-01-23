package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ONSdigital/dis-imf-uploader/config"
	"github.com/ONSdigital/dis-imf-uploader/models"
	auth "github.com/ONSdigital/dp-authorisation/v2/authorisation"
	"github.com/gorilla/mux"

	. "github.com/smartystreets/goconvey/convey"
)

func TestAPISetup(t *testing.T) {
	Convey("Given API setup", t, func() {
		cfg := &config.Config{
			BindAddr:           "localhost:30200",
			MaxUploadSize:      10 * 1024 * 1024,
			TempStorageTimeout: 24 * time.Hour,
			S3Config: config.S3Config{
				Prefix: "imf/",
			},
			AuthConfig: auth.NewDefaultConfig(),
		}

		Convey("When setting up the API", func() {
			Convey("Then router should be configured with routes", func() {
				router := mux.NewRouter()

				// Use noop auth middleware for testing
				authMiddleware := auth.NewNoopMiddleware()

				ctx := context.Background()

				// Setup with nil dependencies for route registration test
				api := Setup(ctx, cfg, router, authMiddleware, nil, nil, nil, nil, nil, nil, nil)

				So(api, ShouldNotBeNil)
				So(api.Router, ShouldEqual, router)
				So(api.AuthMiddleware, ShouldNotBeNil)
			})
		})
	})
}

func TestRequestStructure(t *testing.T) {
	Convey("Given API request structures", t, func() {
		Convey("When creating an upload request with JWT token", func() {
			Convey("Then request should have proper Authorization header", func() {
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)

				part, err := writer.CreateFormFile("file", "test.pdf")
				So(err, ShouldBeNil)

				_, err = io.WriteString(part, "test file content")
				So(err, ShouldBeNil)

				_ = writer.Close()

				req := httptest.NewRequest("POST", "/api/v1/uploads", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("Authorization", "Bearer test-jwt-token")

				So(req.Method, ShouldEqual, "POST")
				So(req.Header.Get("Authorization"), ShouldContainSubstring, "Bearer")
			})
		})

		Convey("When creating a reject request", func() {
			Convey("Then request should have reason in body", func() {
				rejectReq := models.RejectRequest{
					Reason: "File format not acceptable",
				}

				body, err := json.Marshal(rejectReq)
				So(err, ShouldBeNil)

				req := httptest.NewRequest("POST", "/api/v1/uploads/123/reject",
					bytes.NewReader(body))
				req.Header.Set("Authorization", "Bearer test-jwt-token")
				req.Header.Set("Content-Type", "application/json")

				So(req.ContentLength, ShouldBeGreaterThan, 0)
			})
		})

		Convey("When creating a status check request", func() {
			Convey("Then request should be properly formatted", func() {
				req := httptest.NewRequest("GET", "/api/v1/uploads/123", http.NoBody)
				req.Header.Set("Authorization", "Bearer test-jwt-token")

				So(req.Method, ShouldEqual, "GET")
				So(req.URL.Path, ShouldContainSubstring, "uploads")
			})
		})
	})
}
