package api

import (
	"bytes"
	"context"
	"crypto/md5" // #nosec G501 - MD5 used for checksums, not security
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ONSdigital/dis-imf-uploader/models"
	"github.com/ONSdigital/dis-imf-uploader/notifications"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	healthStatusHealthy   = "healthy"
	healthStatusUnhealthy = "unhealthy"
)

// UploadFile handles file upload
func (api *IMFUploaderAPI) UploadFile(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	// Extract user ID from JWT token
	userID, err := api.GetUserID(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{
			Error: "failed to extract user from token",
			Code:  http.StatusUnauthorized,
		})
		return
	}

	// Parse form
	if err := r.ParseMultipartForm(api.cfg.MaxUploadSize); err != nil {
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
	defer func() {
		_ = file.Close()
	}()

	// Check file size
	if fileHeader.Size > api.cfg.MaxUploadSize {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: fmt.Sprintf("file size exceeds max allowed: %d bytes",
				api.cfg.MaxUploadSize),
			Code: http.StatusBadRequest,
		})
		return
	}

	// Calculate checksum and read file
	hash := md5.New() // #nosec G401 - MD5 used for checksums, not security
	fileData := io.TeeReader(file, hash)
	fileBytes, _ := io.ReadAll(fileData)
	checksum := fmt.Sprintf("%x", hash.Sum(nil))

	// Validate file
	if api.cfg.ValidationConfig.Enabled && api.validator != nil {
		validationResult := api.validator.Validate(fileHeader.Filename, fileBytes)
		if !validationResult.Valid {
			_ = api.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
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
	expiresAt := time.Now().Add(api.cfg.TempStorageTimeout)

	err = api.tempStorage.Store(ctx, tempKey, fileBytes)
	if err != nil {
		_ = api.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
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
	exists, _ := api.s3Client.CheckFileExists(ctx, fileHeader.Filename)

	upload := &models.Upload{
		FileName:       fileHeader.Filename,
		FileSize:       fileHeader.Size,
		ContentType:    fileHeader.Header.Get("Content-Type"),
		UploadedBy:     userID,
		FileChecksum:   checksum,
		TempStorageKey: tempKey,
		Status:         models.StatusPending,
		ExpiresAt:      &expiresAt,
	}

	err = api.db.CreateUpload(ctx, upload)
	if err != nil {
		_ = api.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
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
	_ = api.db.LogAudit(ctx, &models.AuditLog{
		UploadID:  upload.ID,
		Action:    "upload",
		UserID:    userID,
		Timestamp: time.Now(),
		Status:    "success",
		Details: map[string]string{
			"file_name": fileHeader.Filename,
			"file_size": fmt.Sprintf("%d", fileHeader.Size),
			"exists":    fmt.Sprintf("%v", exists),
		},
	})

	// Send Slack notification
	_ = api.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
		Type:   "upload",
		Upload: upload,
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
	_ = json.NewEncoder(w).Encode(response)
}

// ApproveUpload approves and uploads to S3
func (api *IMFUploaderAPI) ApproveUpload(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	vars := mux.Vars(r)
	uploadID := vars["id"]

	// Extract user ID from JWT token
	userID, err := api.GetUserID(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{
			Error: "failed to extract user from token",
			Code:  http.StatusUnauthorized,
		})
		return
	}

	upload, err := api.db.GetUpload(ctx, uploadID)
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
	fileBytes, err := api.tempStorage.Get(ctx, upload.TempStorageKey)
	if err != nil {
		_ = api.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
			Type:       "error",
			Upload:     upload,
			Error:      fmt.Sprintf("Temp file not found: %v", err),
			ReviewedBy: userID,
		})

		_ = api.db.LogAudit(ctx, &models.AuditLog{
			UploadID:     upload.ID,
			Action:       "approve",
			UserID:       userID,
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
	exists, _ := api.s3Client.CheckFileExists(ctx, upload.FileName)

	if exists {
		backupKey, err = api.s3Client.BackupFile(ctx, upload.FileName)
		if err != nil {
			_ = api.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
				Type:       "error",
				Upload:     upload,
				Error:      fmt.Sprintf("Failed to backup file: %v", err),
				ReviewedBy: userID,
			})

			writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
				Error: "failed to backup existing file",
				Code:  http.StatusInternalServerError,
			})
			return
		}

		_ = api.db.SaveBackupMetadata(ctx, &models.BackupMetadata{
			OriginalS3Key: upload.FileName,
			BackupS3Key:   backupKey,
			BackupDate:    time.Now(),
			UploadID:      upload.ID,
			Size:          upload.FileSize,
		})
	}

	// Upload to S3
	err = api.s3Client.UploadFile(ctx, upload.FileName,
		bytes.NewReader(fileBytes))
	if err != nil {
		_ = api.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
			Type:       "error",
			Upload:     upload,
			Error:      fmt.Sprintf("S3 upload failed: %v", err),
			ReviewedBy: userID,
		})

		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "S3 upload failed",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	// Invalidate CloudFront (if configured)
	var invID string
	distributionID := api.cfg.DistributionID
	if distributionID != "" {
		invPath := fmt.Sprintf("/%s%s*", api.cfg.S3Config.Prefix, upload.FileName)
		invID, err = api.cloudFrontClient.InvalidateCache(ctx,
			distributionID, []string{invPath})
		if err != nil {
			_ = api.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
				Type:       "error",
				Upload:     upload,
				Error:      fmt.Sprintf("CloudFront invalidation failed: %v", err),
				ReviewedBy: userID,
			})

			writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
				Error: "CloudFront invalidation failed",
				Code:  http.StatusInternalServerError,
			})
			return
		}
	}

	// Purge Cloudflare cache automatically
	if api.cfg.Token != "" && api.cfg.ZoneID != "" {
		cfPath := fmt.Sprintf("/imf/%s", upload.FileName)
		if err := api.cloudflareClient.PurgeCache(ctx, cfPath); err != nil {
			// Log error but don't fail the approval
			_ = api.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
				Type:       "error",
				Upload:     upload,
				Error:      fmt.Sprintf("Cloudflare purge failed (non-critical): %v", err),
				ReviewedBy: userID,
			})
		}
	}

	// Update database
	err = api.db.UpdateUploadApproved(ctx, uploadID, userID,
		upload.FileName, backupKey, invID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to update upload",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	// Clean up temp file
	_ = api.tempStorage.Delete(ctx, upload.TempStorageKey)

	// Log success
	_ = api.db.LogAudit(ctx, &models.AuditLog{
		UploadID:  upload.ID,
		Action:    "approve",
		UserID:    userID,
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
	upload.ReviewedBy = userID
	upload.S3Key = upload.FileName
	upload.BackupS3Key = backupKey
	upload.CloudFrontInvID = invID

	_ = api.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
		Type:       "approve",
		Upload:     upload,
		ReviewedBy: userID,
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
	_ = json.NewEncoder(w).Encode(response)
}

// RejectUpload rejects an upload
func (api *IMFUploaderAPI) RejectUpload(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	uploadID := vars["id"]

	// Extract user ID from JWT token
	userID, err := api.GetUserID(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{
			Error: "failed to extract user from token",
			Code:  http.StatusUnauthorized,
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

	upload, _ := api.db.GetUpload(ctx, uploadID)
	if upload == nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{
			Error: "upload not found",
			Code:  http.StatusNotFound,
		})
		return
	}

	err = api.db.UpdateUploadRejected(ctx, uploadID, userID, req.Reason)
	if err != nil {
		_ = api.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
			Type:       "error",
			Upload:     upload,
			Error:      fmt.Sprintf("Failed to reject upload: %v", err),
			ReviewedBy: userID,
		})

		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to reject upload",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	// Clean up temp file
	_ = api.tempStorage.Delete(ctx, upload.TempStorageKey)

	// Log rejection
	_ = api.db.LogAudit(ctx, &models.AuditLog{
		UploadID:  upload.ID,
		Action:    "reject",
		UserID:    userID,
		Timestamp: time.Now(),
		Status:    "success",
		Details: map[string]string{
			"reason": req.Reason,
		},
	})

	// Send rejection notification
	upload.Status = models.StatusRejected
	upload.ReviewedBy = userID
	_ = api.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
		Type:       "reject",
		Upload:     upload,
		ReviewedBy: userID,
		Reason:     req.Reason,
	})

	response := models.RejectResponse{
		Status: "rejected",
		Reason: req.Reason,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// GetUploadStatus retrieves upload status
func (api *IMFUploaderAPI) GetUploadStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	uploadID := vars["id"]

	upload, _ := api.db.GetUpload(ctx, uploadID)
	if upload == nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{
			Error: "upload not found",
			Code:  http.StatusNotFound,
		})
		return
	}

	// Check CloudFront status if invocation is in progress
	if upload.CloudFrontInvID != "" && upload.InvocationStatus == "InProgress" {
		distributionID := api.cfg.DistributionID
		if distributionID != "" {
			status, _ := api.cloudFrontClient.GetInvalidationStatus(ctx,
				distributionID, upload.CloudFrontInvID)
			upload.InvocationStatus = status

			if status == "Completed" {
				_ = api.db.UpdateCloudFrontStatus(ctx, uploadID, "Completed")
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(upload)
}

// PurgeCloudflareCache manually purges Cloudflare cache
func (api *IMFUploaderAPI) PurgeCloudflareCache(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	uploadID := vars["id"]

	// Extract user ID from JWT token
	userID, err := api.GetUserID(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{
			Error: "failed to extract user from token",
			Code:  http.StatusUnauthorized,
		})
		return
	}

	upload, _ := api.db.GetUpload(ctx, uploadID)
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
	err = api.cloudflareClient.PurgeCache(ctx, cfPath)
	if err != nil {
		_ = api.slackNotifier.Notify(ctx, &notifications.NotificationEvent{
			Type:       "error",
			Upload:     upload,
			Error:      fmt.Sprintf("Cloudflare purge failed: %v", err),
			ReviewedBy: userID,
		})

		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "Cloudflare purge failed",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	_ = api.db.LogAudit(ctx, &models.AuditLog{
		UploadID:  upload.ID,
		Action:    "purge_cloudflare",
		UserID:    userID,
		Timestamp: time.Now(),
		Status:    "success",
	})

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Cloudflare cache purged successfully",
	})
}

// Note: Authorization is handled via dp-permissions-api through authMiddleware.Require()
// No need for local role checking

// ListUploads retrieves uploads with filtering and pagination
func (api *IMFUploaderAPI) ListUploads(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	status := r.URL.Query().Get("status")
	page := 1
	pageSize := 20

	if p := r.URL.Query().Get("page"); p != "" {
		_, _ = fmt.Sscanf(p, "%d", &page)
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		_, _ = fmt.Sscanf(ps, "%d", &pageSize)
		if pageSize > 100 {
			pageSize = 100
		}
	}

	sortBy := r.URL.Query().Get("sort_by")
	sortDir := r.URL.Query().Get("sort_dir")

	uploads, total, err := api.db.ListUploads(ctx, status, page, pageSize, sortBy, sortDir)
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

// ListAuditLogs retrieves audit logs with filtering
func (api *IMFUploaderAPI) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	uploadID := r.URL.Query().Get("upload_id")
	action := r.URL.Query().Get("action")
	userEmail := r.URL.Query().Get("user_email")

	page := 1
	pageSize := 50

	if p := r.URL.Query().Get("page"); p != "" {
		_, _ = fmt.Sscanf(p, "%d", &page)
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		_, _ = fmt.Sscanf(ps, "%d", &pageSize)
		if pageSize > 200 {
			pageSize = 200
		}
	}

	logs, total, err := api.db.ListAuditLogs(ctx, uploadID, action, userEmail, page, pageSize)
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
func (api *IMFUploaderAPI) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	dependencies := make(map[string]string)

	// Check MongoDB
	if err := api.db.HealthCheck(ctx); err != nil {
		dependencies["mongodb"] = healthStatusUnhealthy
	} else {
		dependencies["mongodb"] = healthStatusHealthy
	}

	// Check temp storage
	testKey := "health_check_test"
	if err := api.tempStorage.Store(ctx, testKey, []byte("test")); err != nil {
		dependencies["temp_storage"] = healthStatusUnhealthy
	} else {
		_ = api.tempStorage.Delete(ctx, testKey)
		dependencies["temp_storage"] = healthStatusHealthy
	}

	status := healthStatusHealthy
	for _, v := range dependencies {
		if v == healthStatusUnhealthy {
			status = healthStatusUnhealthy
			break
		}
	}

	statusCode := http.StatusOK
	if status == healthStatusUnhealthy {
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
	_ = json.NewEncoder(w).Encode(data)
}
