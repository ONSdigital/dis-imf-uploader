package api

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ONSdigital/dis-imf-uploader/config"
	"github.com/ONSdigital/dis-imf-uploader/models"
	"github.com/ONSdigital/dis-imf-uploader/mongo"
	"github.com/ONSdigital/dis-imf-uploader/notifications"
	"github.com/ONSdigital/dis-imf-uploader/storage"
	"github.com/ONSdigital/dis-imf-uploader/temp"
	"github.com/ONSdigital/dis-imf-uploader/validation"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Handler struct {
	cfg              *config.Config
	db               *mongo.DB
	s3Client         *storage.S3Client
	cloudFrontClient *storage.CloudFrontClient
	cloudflareClient *storage.CloudflareClient
	slackNotifier    *notifications.SlackNotifier
	tempStorage      temp.Storage
	validator        *validation.FileValidator
}

func NewHandler(
	cfg *config.Config,
	db *mongo.DB,
	s3 *storage.S3Client,
	cf *storage.CloudFrontClient,
	cflare *storage.CloudflareClient,
	slack *notifications.SlackNotifier,
	ts temp.Storage,
	validator *validation.FileValidator,
) *Handler {
	return &Handler{
		cfg:              cfg,
		db:               db,
		s3Client:         s3,
		cloudFrontClient: cf,
		cloudflareClient: cflare,
		slackNotifier:    slack,
		tempStorage:      ts,
		validator:        validator,
	}
}

// UploadFile handles file upload
func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	// Parse form
	if err := r.ParseMultipartForm(h.cfg.MaxUploadSize); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "failed to parse form",
			Code:  http.StatusBadRequest,
		})
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "no file provided",
			Code:  http.StatusBadRequest,
		})
		return
	}
	defer file.Close()

	// Check file size
	if fileHeader.Size > h.cfg.MaxUploadSize {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: fmt.Sprintf("file size exceeds max allowed: %d bytes",
				h.cfg.MaxUploadSize),
			Code: http.StatusBadRequest,
		})
		return
	}

	userEmail := r.Header.Get("X-User-Email")

	// Calculate checksum and read file
	hash := md5.New()
	fileData := io.TeeReader(file, hash)
	fileBytes, _ := io.ReadAll(fileData)
	checksum := fmt.Sprintf("%x", hash.Sum(nil))

	// Validate file
	if h.cfg.ValidationConfig.Enabled && h.validator != nil {
		validationResult := h.validator.Validate(fileHeader.Filename, fileBytes)
		if !validationResult.Valid {
			h.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
				Type:  "error",
				Error: fmt.Sprintf("File validation failed for %s: %s", fileHeader.Filename, validationResult.GetErrorMessage()),
			})

			writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
				Error: fmt.Sprintf("File validation failed: %s", validationResult.GetErrorMessage()),
				Code:  http.StatusBadRequest,
			})
			return
		}
	}

	// Store in temp storage
	tempKey := primitive.NewObjectID().Hex()
	expiresAt := time.Now().Add(h.cfg.TempStorageTimeout)

	err = h.tempStorage.Store(ctx, tempKey, fileBytes)
	if err != nil {
		h.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
			Type:  "error",
			Error: fmt.Sprintf("Failed to store temp file: %v", err),
		})

		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to store file",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	// Check if file exists in S3
	exists, _ := h.s3Client.CheckFileExists(ctx, fileHeader.Filename)

	upload := &models.Upload{
		FileName:       fileHeader.Filename,
		FileSize:       fileHeader.Size,
		ContentType:    fileHeader.Header.Get("Content-Type"),
		UploadedBy:     userEmail,
		FileChecksum:   checksum,
		TempStorageKey: tempKey,
		Status:         models.StatusPending,
		ExpiresAt:      &expiresAt,
	}

	err = h.db.CreateUpload(ctx, upload)
	if err != nil {
		h.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
			Type:  "error",
			Error: fmt.Sprintf("Failed to create upload record: %v", err),
		})

		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to create upload record",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	// Log audit
	h.db.LogAudit(ctx, &models.AuditLog{
		UploadID:  upload.ID,
		Action:    "upload",
		UserEmail: userEmail,
		Timestamp: time.Now(),
		Status:    "success",
		Details: map[string]string{
			"file_name": fileHeader.Filename,
			"file_size": fmt.Sprintf("%d", fileHeader.Size),
			"exists":    fmt.Sprintf("%v", exists),
		},
	})

	// Send Slack notification
	user, _ := h.db.GetUserByEmail(ctx, userEmail)
	h.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
		Type:   "upload",
		Upload: upload,
		User:   user,
	})

	response := models.UploadResponse{
		ID:           upload.ID,
		FileName:     upload.FileName,
		Status:       upload.Status,
		ExistsInS3:   exists,
		FileChecksum: checksum,
		Message:      "File pending review",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(response)
}

// ApproveUpload approves and uploads to S3
func (h *Handler) ApproveUpload(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	vars := mux.Vars(r)
	uploadID := vars["id"]
	reviewerEmail := r.Header.Get("X-User-Email")

	if !h.isReviewer(ctx, reviewerEmail) {
		writeJSON(w, http.StatusForbidden, models.ErrorResponse{
			Error: "unauthorized",
			Code:  http.StatusForbidden,
		})
		return
	}

	upload, err := h.db.GetUpload(ctx, uploadID)
	if err != nil || upload == nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{
			Error: "upload not found",
			Code:  http.StatusNotFound,
		})
		return
	}

	if upload.Status != models.StatusPending {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "only pending uploads can be approved",
			Code:  http.StatusBadRequest,
		})
		return
	}

	// Retrieve temp file
	fileBytes, err := h.tempStorage.Get(ctx, upload.TempStorageKey)
	if err != nil {
		h.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
			Type:       "error",
			Upload:     upload,
			Error:      fmt.Sprintf("Temp file not found: %v", err),
			ReviewedBy: reviewerEmail,
		})

		h.db.LogAudit(ctx, &models.AuditLog{
			UploadID:     upload.ID,
			Action:       "approve",
			UserEmail:    reviewerEmail,
			Timestamp:    time.Now(),
			Status:       "failure",
			ErrorMessage: "temp file not found",
		})

		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "temp file not found",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	// Check if file exists and backup if needed
	var backupKey string
	exists, _ := h.s3Client.CheckFileExists(ctx, upload.FileName)

	if exists {
		backupKey, err = h.s3Client.BackupFile(ctx, upload.FileName)
		if err != nil {
			h.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
				Type:       "error",
				Upload:     upload,
				Error:      fmt.Sprintf("Failed to backup file: %v", err),
				ReviewedBy: reviewerEmail,
			})

			writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
				Error: "failed to backup existing file",
				Code:  http.StatusInternalServerError,
			})
			return
		}

		h.db.SaveBackupMetadata(ctx, &models.BackupMetadata{
			OriginalS3Key: upload.FileName,
			BackupS3Key:   backupKey,
			BackupDate:    time.Now(),
			UploadID:      upload.ID,
			Size:          upload.FileSize,
		})
	}

	// Upload to S3
	err = h.s3Client.UploadFile(ctx, upload.FileName,
		bytes.NewReader(fileBytes))
	if err != nil {
		h.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
			Type:       "error",
			Upload:     upload,
			Error:      fmt.Sprintf("S3 upload failed: %v", err),
			ReviewedBy: reviewerEmail,
		})

		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "S3 upload failed",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	// Invalidate CloudFront (if configured)
	var invID string
	distributionID := h.cfg.CloudFrontConfig.DistributionID
	if distributionID != "" {
		invPath := fmt.Sprintf("/%s%s*", h.cfg.S3Config.Prefix, upload.FileName)
		invID, err = h.cloudFrontClient.InvalidateCache(ctx,
			distributionID, []string{invPath})
		if err != nil {
			h.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
				Type:       "error",
				Upload:     upload,
				Error:      fmt.Sprintf("CloudFront invalidation failed: %v", err),
				ReviewedBy: reviewerEmail,
			})

			writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
				Error: "CloudFront invalidation failed",
				Code:  http.StatusInternalServerError,
			})
			return
		}
	}

	// Purge Cloudflare cache automatically
	if h.cfg.CloudflareConfig.Token != "" && h.cfg.CloudflareConfig.ZoneID != "" {
		cfPath := fmt.Sprintf("/imf/%s", upload.FileName)
		if err := h.cloudflareClient.PurgeCache(ctx, cfPath); err != nil {
			// Log error but don't fail the approval
			h.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
				Type:       "error",
				Upload:     upload,
				Error:      fmt.Sprintf("Cloudflare purge failed (non-critical): %v", err),
				ReviewedBy: reviewerEmail,
			})
		}
	}

	// Update database
	err = h.db.UpdateUploadApproved(ctx, uploadID, reviewerEmail,
		upload.FileName, backupKey, invID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to update upload",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	// Clean up temp file
	h.tempStorage.Delete(ctx, upload.TempStorageKey)

	// Log success
	h.db.LogAudit(ctx, &models.AuditLog{
		UploadID:  upload.ID,
		Action:    "approve",
		UserEmail: reviewerEmail,
		Timestamp: time.Now(),
		Status:    "success",
		Details: map[string]string{
			"s3_key":           upload.FileName,
			"backup_key":       backupKey,
			"cloudfront_inv":   invID,
			"cloudflare_ready": "true",
		},
	})

	// Send approval notification
	upload.Status = models.StatusApproved
	upload.ReviewedBy = reviewerEmail
	upload.S3Key = upload.FileName
	upload.BackupS3Key = backupKey
	upload.CloudFrontInvID = invID

	h.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
		Type:       "approve",
		Upload:     upload,
		ReviewedBy: reviewerEmail,
	})

	response := models.ApproveResponse{
		ID:              upload.ID,
		Status:          models.StatusApproved,
		S3Key:           upload.FileName,
		BackupKey:       backupKey,
		CloudFrontInvID: invID,
		CloudflareReady: true,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// RejectUpload rejects an upload
func (h *Handler) RejectUpload(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	uploadID := vars["id"]
	reviewerEmail := r.Header.Get("X-User-Email")

	if !h.isReviewer(ctx, reviewerEmail) {
		writeJSON(w, http.StatusForbidden, models.ErrorResponse{
			Error: "unauthorized",
			Code:  http.StatusForbidden,
		})
		return
	}

	var req models.RejectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid request body",
			Code:  http.StatusBadRequest,
		})
		return
	}

	upload, _ := h.db.GetUpload(ctx, uploadID)
	if upload == nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{
			Error: "upload not found",
			Code:  http.StatusNotFound,
		})
		return
	}

	err := h.db.UpdateUploadRejected(ctx, uploadID, reviewerEmail, req.Reason)
	if err != nil {
		h.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
			Type:       "error",
			Upload:     upload,
			Error:      fmt.Sprintf("Failed to reject upload: %v", err),
			ReviewedBy: reviewerEmail,
		})

		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to reject upload",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	// Clean up temp file
	h.tempStorage.Delete(ctx, upload.TempStorageKey)

	// Log rejection
	h.db.LogAudit(ctx, &models.AuditLog{
		UploadID:  upload.ID,
		Action:    "reject",
		UserEmail: reviewerEmail,
		Timestamp: time.Now(),
		Status:    "success",
		Details: map[string]string{
			"reason": req.Reason,
		},
	})

	// Send rejection notification
	upload.Status = models.StatusRejected
	upload.ReviewedBy = reviewerEmail
	h.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
		Type:       "reject",
		Upload:     upload,
		ReviewedBy: reviewerEmail,
		Reason:     req.Reason,
	})

	response := models.RejectResponse{
		Status: "rejected",
		Reason: req.Reason,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// GetUploadStatus retrieves upload status
func (h *Handler) GetUploadStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	uploadID := vars["id"]

	upload, _ := h.db.GetUpload(ctx, uploadID)
	if upload == nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{
			Error: "upload not found",
			Code:  http.StatusNotFound,
		})
		return
	}

	// Check CloudFront status if invocation is in progress
	if upload.CloudFrontInvID != "" && upload.InvocationStatus == "InProgress" {
		distributionID := h.cfg.CloudFrontConfig.DistributionID
		if distributionID != "" {
			status, _ := h.cloudFrontClient.GetInvalidationStatus(ctx,
				distributionID, upload.CloudFrontInvID)
			upload.InvocationStatus = status

			if status == "Completed" {
				h.db.UpdateCloudFrontStatus(ctx, uploadID, "Completed")
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(upload)
}

// PurgeCloudflareCache manually purges Cloudflare cache
func (h *Handler) PurgeCloudflareCache(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	uploadID := vars["id"]
	reviewerEmail := r.Header.Get("X-User-Email")

	upload, _ := h.db.GetUpload(ctx, uploadID)
	if upload == nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{
			Error: "upload not found",
			Code:  http.StatusNotFound,
		})
		return
	}

	if upload.Status != models.StatusApproved {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "only approved uploads can purge Cloudflare",
			Code:  http.StatusBadRequest,
		})
		return
	}

	// Purge Cloudflare
	cfPath := fmt.Sprintf("/imf/%s", upload.FileName)
	err := h.cloudflareClient.PurgeCache(ctx, cfPath)
	if err != nil {
		h.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
			Type:       "error",
			Upload:     upload,
			Error:      fmt.Sprintf("Cloudflare purge failed: %v", err),
			ReviewedBy: reviewerEmail,
		})

		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "Cloudflare purge failed",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	h.db.LogAudit(ctx, &models.AuditLog{
		UploadID:  upload.ID,
		Action:    "purge_cloudflare",
		UserEmail: reviewerEmail,
		Timestamp: time.Now(),
		Status:    "success",
	})

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Cloudflare cache purged successfully",
	})
}

// isReviewer checks if user has reviewer role
func (h *Handler) isReviewer(ctx context.Context, email string) bool {
	user, err := h.db.GetUserByEmail(ctx, email)
	if err != nil || user == nil {
		return false
	}
	return user.Role == models.RoleReviewer || user.Role == models.RoleAdmin
}

// ListUploads retrieves uploads with filtering and pagination
func (h *Handler) ListUploads(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	status := r.URL.Query().Get("status")
	page := 1
	pageSize := 20

	if p := r.URL.Query().Get("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		fmt.Sscanf(ps, "%d", &pageSize)
		if pageSize > 100 {
			pageSize = 100
		}
	}

	sortBy := r.URL.Query().Get("sort_by")
	sortDir := r.URL.Query().Get("sort_dir")

	uploads, total, err := h.db.ListUploads(ctx, status, page, pageSize, sortBy, sortDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to list uploads",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize != 0 {
		totalPages++
	}

	response := models.ListUploadsResponse{
		Uploads:    uploads,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}

	writeJSON(w, http.StatusOK, response)
}

// CreateUser creates a new user
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid request body",
			Code:  http.StatusBadRequest,
		})
		return
	}

	user := &models.User{
		Email: req.Email,
		Name:  req.Name,
		Role:  req.Role,
	}

	if err := h.db.CreateUser(ctx, user); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to create user",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

// GetUser retrieves a user by ID
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	userID := vars["id"]

	user, err := h.db.GetUser(ctx, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to get user",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	if user == nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{
			Error: "user not found",
			Code:  http.StatusNotFound,
		})
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// UpdateUser updates a user
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	userID := vars["id"]

	var req models.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid request body",
			Code:  http.StatusBadRequest,
		})
		return
	}

	if err := h.db.UpdateUser(ctx, userID, req.Name, req.Role); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to update user",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "user updated"})
}

// DeleteUser deletes a user
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	userID := vars["id"]

	if err := h.db.DeleteUser(ctx, userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to delete user",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "user deleted"})
}

// ListAuditLogs retrieves audit logs with filtering
func (h *Handler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	uploadID := r.URL.Query().Get("upload_id")
	action := r.URL.Query().Get("action")
	userEmail := r.URL.Query().Get("user_email")

	page := 1
	pageSize := 50

	if p := r.URL.Query().Get("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		fmt.Sscanf(ps, "%d", &pageSize)
		if pageSize > 200 {
			pageSize = 200
		}
	}

	logs, total, err := h.db.ListAuditLogs(ctx, uploadID, action, userEmail, page, pageSize)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to list audit logs",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize != 0 {
		totalPages++
	}

	response := models.ListAuditLogsResponse{
		Logs:       logs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}

	writeJSON(w, http.StatusOK, response)
}

// HealthCheck performs health check on all dependencies
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	dependencies := make(map[string]string)

	// Check MongoDB
	if err := h.db.HealthCheck(ctx); err != nil {
		dependencies["mongodb"] = "unhealthy"
	} else {
		dependencies["mongodb"] = "healthy"
	}

	// Check temp storage
	testKey := "health_check_test"
	if err := h.tempStorage.Store(ctx, testKey, []byte("test")); err != nil {
		dependencies["temp_storage"] = "unhealthy"
	} else {
		h.tempStorage.Delete(ctx, testKey)
		dependencies["temp_storage"] = "healthy"
	}

	status := "healthy"
	for _, v := range dependencies {
		if v == "unhealthy" {
			status = "unhealthy"
			break
		}
	}

	statusCode := http.StatusOK
	if status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	response := models.HealthCheckResponse{
		Status:       status,
		Version:      "1.0.0",
		Dependencies: dependencies,
	}

	writeJSON(w, statusCode, response)
}

// Helper function to write JSON responses
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// Helper function to get distribution ID based on environment variable
func getDistributionID(cfg *config.Config) string {
	return cfg.CloudFrontConfig.DistributionID
}
