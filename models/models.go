package models

import (
	"time"
)

// UploadStatus represents the current state of an upload.
type UploadStatus string

const (
	// StatusPending indicates an upload is awaiting review.
	StatusPending UploadStatus = "pending"
	// StatusApproved indicates an upload has been approved.
	StatusApproved UploadStatus = "approved"
	// StatusRejected indicates an upload has been rejected.
	StatusRejected UploadStatus = "rejected"
	// StatusFailed indicates an upload has failed processing.
	StatusFailed UploadStatus = "failed"
)

// Upload represents a file upload record in the database.
type Upload struct {
	ID               string       `bson:"_id,omitempty"`
	FileName         string       `bson:"file_name"`
	FileSize         int64        `bson:"file_size"`
	ContentType      string       `bson:"content_type"`
	UploadedBy       string       `bson:"uploaded_by"`
	UploadedAt       time.Time    `bson:"uploaded_at"`
	Status           UploadStatus `bson:"status"`
	ReviewedBy       string       `bson:"reviewed_by,omitempty"`
	ReviewedAt       *time.Time   `bson:"reviewed_at,omitempty"`
	S3Key            string       `bson:"s3_key,omitempty"`
	BackupS3Key      string       `bson:"backup_s3_key,omitempty"`
	RejectionReason  string       `bson:"rejection_reason,omitempty"`
	CloudFrontInvID  string       `bson:"cloudfront_inv_id,omitempty"`
	InvocationStatus string       `bson:"invocation_status,omitempty"`
	FileChecksum     string       `bson:"file_checksum"`
	TempStorageKey   string       `bson:"temp_storage_key,omitempty"`
	ExpiresAt        *time.Time   `bson:"expires_at,omitempty"`
}

// BackupMetadata represents metadata for a backup file.
type BackupMetadata struct {
	ID            string    `bson:"_id,omitempty"`
	OriginalS3Key string    `bson:"original_s3_key"`
	BackupS3Key   string    `bson:"backup_s3_key"`
	BackupDate    time.Time `bson:"backup_date"`
	UploadID      string    `bson:"upload_id"`
	Size          int64     `bson:"size"`
}

// AuditLog represents an audit trail entry for upload actions.
type AuditLog struct {
	ID           string            `bson:"_id,omitempty"`
	UploadID     string            `bson:"upload_id"`
	Action       string            `bson:"action"`
	UserID       string            `bson:"user_id"`
	Timestamp    time.Time         `bson:"timestamp"`
	Details      map[string]string `bson:"details,omitempty"`
	Status       string            `bson:"status"`
	ErrorMessage string            `bson:"error_message,omitempty"`
}

// UploadRequest represents the request body for uploading a file.
type UploadRequest struct {
	FileName string `form:"file_name" binding:"required"`
	File     []byte `form:"file" binding:"required"`
}

// UploadResponse represents the response after uploading a file.
type UploadResponse struct {
	ID           string       `json:"id"`
	FileName     string       `json:"file_name"`
	Status       UploadStatus `json:"status"`
	ExistsInS3   bool         `json:"exists_in_s3"`
	FileChecksum string       `json:"file_checksum"`
	Message      string       `json:"message"`
}

// ApproveResponse represents the response after approving an upload.
type ApproveResponse struct {
	ID              string       `json:"id"`
	Status          UploadStatus `json:"status"`
	S3Key           string       `json:"s3_key"`
	BackupKey       string       `json:"backup_key,omitempty"`
	CloudFrontInvID string       `json:"cloudfront_inv_id,omitempty"`
	CloudflareReady bool         `json:"cloudflare_ready"`
}

// RejectRequest represents the request body for rejecting an upload.
type RejectRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// RejectResponse represents the response after rejecting an upload.
type RejectResponse struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

// ListUploadsRequest represents the request parameters for listing uploads.
type ListUploadsRequest struct {
	Status   string `form:"status"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	SortBy   string `form:"sort_by"`
	SortDir  string `form:"sort_dir"`
}

// ListUploadsResponse represents the response for listing uploads.
type ListUploadsResponse struct {
	Uploads    []*Upload `json:"uploads"`
	Total      int64     `json:"total"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
	TotalPages int       `json:"total_pages"`
}

// ListAuditLogsRequest represents the request parameters for listing audit
// logs.
type ListAuditLogsRequest struct {
	UploadID  string `form:"upload_id"`
	Action    string `form:"action"`
	UserEmail string `form:"user_email"`
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
}

// ListAuditLogsResponse represents the response for listing audit logs.
type ListAuditLogsResponse struct {
	Logs       []*AuditLog `json:"logs"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// HealthCheckResponse represents the response for health check endpoints.
type HealthCheckResponse struct {
	Status       string            `json:"status"`
	Version      string            `json:"version"`
	Dependencies map[string]string `json:"dependencies"`
}
