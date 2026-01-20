package api

import (
	"github.com/ONSdigital/dis-imf-uploader/config"
	"github.com/ONSdigital/dis-imf-uploader/mongo"
	"github.com/ONSdigital/dis-imf-uploader/notifications"
	"github.com/ONSdigital/dis-imf-uploader/storage"
	"github.com/ONSdigital/dis-imf-uploader/temp"
	"github.com/ONSdigital/dis-imf-uploader/validation"
	"github.com/gorilla/mux"
)

func SetupRoutes(
	router *mux.Router,
	cfg *config.Config,
	db *mongo.DB,
	s3 *storage.S3Client,
	cf *storage.CloudFrontClient,
	cflare *storage.CloudflareClient,
	slack *notifications.SlackNotifier,
	ts temp.Storage,
	validator *validation.FileValidator,
) {
	handler := NewHandler(cfg, db, s3, cf, cflare, slack, ts, validator)

	// Apply middleware
	router.Use(LoggingMiddleware)
	router.Use(AuthMiddleware(cfg))
	router.Use(UserContextMiddleware(db))

	// API routes - Uploads
	router.HandleFunc("/api/v1/uploads", handler.ListUploads).Methods("GET")
	router.HandleFunc("/api/v1/uploads", handler.UploadFile).Methods("POST")
	router.HandleFunc("/api/v1/uploads/{id}", handler.GetUploadStatus).Methods("GET")
	router.HandleFunc("/api/v1/uploads/{id}/approve", handler.ApproveUpload).Methods("POST")
	router.HandleFunc("/api/v1/uploads/{id}/reject", handler.RejectUpload).Methods("POST")
	router.HandleFunc("/api/v1/uploads/{id}/purge-cloudflare",
		handler.PurgeCloudflareCache).Methods("POST")

	// API routes - Users
	router.HandleFunc("/api/v1/users", handler.CreateUser).Methods("POST")
	router.HandleFunc("/api/v1/users/{id}", handler.GetUser).Methods("GET")
	router.HandleFunc("/api/v1/users/{id}", handler.UpdateUser).Methods("PUT")
	router.HandleFunc("/api/v1/users/{id}", handler.DeleteUser).Methods("DELETE")

	// API routes - Audit Logs
	router.HandleFunc("/api/v1/audit-logs", handler.ListAuditLogs).Methods("GET")

	// Health check
	router.HandleFunc("/health", handler.HealthCheck).Methods("GET")
}
