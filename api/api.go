package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/ONSdigital/dis-imf-uploader/config"
	"github.com/ONSdigital/dis-imf-uploader/mongo"
	"github.com/ONSdigital/dis-imf-uploader/notifications"
	"github.com/ONSdigital/dis-imf-uploader/storage"
	"github.com/ONSdigital/dis-imf-uploader/temp"
	"github.com/ONSdigital/dis-imf-uploader/validation"
	auth "github.com/ONSdigital/dp-authorisation/v2/authorisation"
	dprequest "github.com/ONSdigital/dp-net/v2/request"
	"github.com/ONSdigital/log.go/v2/log"
	"github.com/gorilla/mux"
)

// IMFUploaderAPI provides a struct to wrap the api around
type IMFUploaderAPI struct {
	Router           *mux.Router
	AuthMiddleware   auth.Middleware
	cfg              *config.Config
	db               mongo.Store
	s3Client         *storage.S3Client
	cloudFrontClient *storage.CloudFrontClient
	cloudflareClient *storage.CloudflareClient
	slackNotifier    *notifications.SlackNotifier
	tempStorage      temp.Storage
	validator        *validation.FileValidator
}

// Setup initializes and configures the API routes and middleware
func Setup(
	_ context.Context,
	cfg *config.Config,
	router *mux.Router,
	authMiddleware auth.Middleware,
	db mongo.Store,
	s3 *storage.S3Client,
	cf *storage.CloudFrontClient,
	cflare *storage.CloudflareClient,
	slack *notifications.SlackNotifier,
	ts temp.Storage,
	validator *validation.FileValidator,
) *IMFUploaderAPI {
	api := &IMFUploaderAPI{
		Router:           router,
		AuthMiddleware:   authMiddleware,
		cfg:              cfg,
		db:               db,
		s3Client:         s3,
		cloudFrontClient: cf,
		cloudflareClient: cflare,
		slackNotifier:    slack,
		tempStorage:      ts,
		validator:        validator,
	}

	// Define API routes with authentication and authorization
	api.post("/api/v1/uploads",
		authMiddleware.Require("imf:upload", api.UploadFile),
	)

	api.get("/api/v1/uploads",
		authMiddleware.Require("imf:read", api.ListUploads),
	)

	api.get("/api/v1/uploads/{id}",
		authMiddleware.Require("imf:read", api.GetUploadStatus),
	)

	api.post("/api/v1/uploads/{id}/approve",
		authMiddleware.Require("imf:approve", api.ApproveUpload),
	)

	api.post("/api/v1/uploads/{id}/reject",
		authMiddleware.Require("imf:reject", api.RejectUpload),
	)

	api.post("/api/v1/uploads/{id}/purge-cloudflare",
		authMiddleware.Require("imf:purge", api.PurgeCloudflareCache),
	)

	api.get("/api/v1/audit-logs",
		authMiddleware.Require("imf:read", api.ListAuditLogs),
	)

	// Health check - no auth required
	router.HandleFunc("/health", api.HealthCheck).Methods("GET")

	return api
}

// Helper methods for registering HTTP methods
func (api *IMFUploaderAPI) get(path string, handler http.HandlerFunc) {
	api.Router.HandleFunc(path, handler).Methods("GET")
}

func (api *IMFUploaderAPI) post(path string, handler http.HandlerFunc) {
	api.Router.HandleFunc(path, handler).Methods("POST")
}

// GetUserID extracts the user ID from the Authorization header by parsing
// the JWT token. Returns the user ID or an empty string if the token
// cannot be parsed.
func (api *IMFUploaderAPI) GetUserID(r *http.Request) (string, error) {
	bearerToken := r.Header.Get(dprequest.AuthHeaderKey)
	if bearerToken == "" {
		return "", errors.New("authorization header missing")
	}

	// Remove "Bearer " prefix if present
	bearerToken = strings.TrimPrefix(bearerToken, dprequest.BearerPrefix)

	// Parse the JWT token to extract user information
	entityData, err := api.AuthMiddleware.Parse(bearerToken)
	if err != nil {
		log.Warn(r.Context(), "failed to parse JWT token for user ID extraction", log.Data{
			"error": err.Error(),
		})
		return "", err
	}

	if entityData.UserID == "" {
		return "", errors.New("token valid but user ID claim is empty")
	}

	return entityData.UserID, nil
}
