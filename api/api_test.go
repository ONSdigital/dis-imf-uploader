package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ONSdigital/dis-imf-uploader/config"
	authorisationMock "github.com/ONSdigital/dp-authorisation/v2/authorisation/mock"

	"github.com/gorilla/mux"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSetup(t *testing.T) {
	mockAuthMiddleware := &authorisationMock.MiddlewareMock{
		RequireFunc: func(permission string, handlerFunc http.HandlerFunc) http.HandlerFunc {
			return handlerFunc
		},
		CloseFunc: func(ctx context.Context) error {
			return nil
		},
	}

	Convey("Given an API instance", t, func() {
		r := mux.NewRouter()
		ctx := context.Background()
		cfg := &config.Config{
			MaxUploadSize: 10 * 1024 * 1024,
		}

		api := Setup(ctx, cfg, r, mockAuthMiddleware, nil, nil, nil, nil, nil, nil, nil)

		Convey("When created the following routes should have been added", func() {
			So(api, ShouldNotBeNil)
			So(api.Router, ShouldEqual, r)
			So(api.AuthMiddleware, ShouldNotBeNil)

			Convey("Then upload routes should be configured", func() {
				So(hasRoute(api.Router, "/api/v1/uploads", "POST"), ShouldBeTrue)
				So(hasRoute(api.Router, "/api/v1/uploads", "GET"), ShouldBeTrue)
				So(hasRoute(api.Router, "/api/v1/uploads/123", "GET"), ShouldBeTrue)
			})

			Convey("Then approval routes should be configured", func() {
				So(hasRoute(api.Router, "/api/v1/uploads/123/approve", "POST"), ShouldBeTrue)
				So(hasRoute(api.Router, "/api/v1/uploads/123/reject", "POST"), ShouldBeTrue)
			})

			Convey("Then cache purge route should be configured", func() {
				So(hasRoute(api.Router, "/api/v1/uploads/123/purge-cloudflare", "POST"), ShouldBeTrue)
			})

			Convey("Then audit log route should be configured", func() {
				So(hasRoute(api.Router, "/api/v1/audit-logs", "GET"), ShouldBeTrue)
			})

			Convey("Then health check route should be configured", func() {
				So(hasRoute(api.Router, "/health", "GET"), ShouldBeTrue)
			})
		})
	})
}

// hasRoute checks if a route with the given path and method exists in the router
func hasRoute(r *mux.Router, path, method string) bool {
	req := httptest.NewRequest(method, path, http.NoBody)
	match := &mux.RouteMatch{}
	return r.Match(req, match)
}
