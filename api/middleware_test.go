package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ONSdigital/dis-imf-uploader/config"

	. "github.com/smartystreets/goconvey/convey"
)

func TestAuthMiddleware(t *testing.T) {
	Convey("Given auth middleware", t, func() {
		cfg := &config.Config{
			ServiceAuthToken: "test-token-123",
		}

		middleware := AuthMiddleware(cfg)
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		})

		Convey("When request has valid bearer token", func() {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", "Bearer test-token-123")
			w := httptest.NewRecorder()

			middleware(testHandler).ServeHTTP(w, req)

			Convey("Then request should be allowed", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				So(w.Body.String(), ShouldEqual, "success")
			})
		})

		Convey("When request has no authorization header", func() {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			middleware(testHandler).ServeHTTP(w, req)

			Convey("Then request should be denied", func() {
				So(w.Code, ShouldEqual, http.StatusUnauthorized)
			})
		})

		Convey("When request has invalid token", func() {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", "Bearer invalid-token")
			w := httptest.NewRecorder()

			middleware(testHandler).ServeHTTP(w, req)

			Convey("Then request should be denied", func() {
				So(w.Code, ShouldEqual, http.StatusUnauthorized)
			})
		})
	})
}
