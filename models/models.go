package models

import (
	"time"
)

type UploadStatus string

const (
	StatusPending  UploadStatus = "pending"
	StatusApproved UploadStatus = "approved"
	StatusRejected UploadStatus = "rejected"
	StatusFailed   UploadStatus = "failed"
)

type UserRole string

const (
	RoleUploader UserRole = "uploader"
	RoleReviewer UserRole = "reviewer"
	RoleAdmin    UserRole = "admin"
)

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

type User struct {
	ID        string    `bson:"_id,omitempty"`
	Email     string    `bson:"email"`
	Name      string    `bson:"name"`
	Role      UserRole  `bson:"role"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

type BackupMetadata struct {
	ID            string    `bson:"_id,omitempty"`
	OriginalS3Key string    `bson:"original_s3_key"`
	BackupS3Key   string    `bson:"backup_s3_key"`
	BackupDate    time.Time `bson:"backup_date"`
	UploadID      string    `bson:"upload_id"`
	Size          int64     `bson:"size"`
}

type AuditLog struct {
	ID           string            `bson:"_id,omitempty"`
	UploadID     string            `bson:"upload_id"`
	Action       string            `bson:"action"`
	UserEmail    string            `bson:"user_email"`
	Timestamp    time.Time         `bson:"timestamp"`
	Details      map[string]string `bson:"details,omitempty"`
	Status       string            `bson:"status"`
	ErrorMessage string            `bson:"error_message,omitempty"`
}

// API Request/Response Models
type UploadRequest struct {
	FileName string `form:"file_name" binding:"required"`
	File     []byte `form:"file" binding:"required"`
}

type UploadResponse struct {
	ID           string       `json:"id"`
	FileName     string       `json:"file_name"`
	Status       UploadStatus `json:"status"`
	ExistsInS3   bool         `json:"exists_in_s3"`
	FileChecksum string       `json:"file_checksum"`
	Message      string       `json:"message"`
}

type ApproveResponse struct {
	ID              string       `json:"id"`
	Status          UploadStatus `json:"status"`
	S3Key           string       `json:"s3_key"`
	BackupKey       string       `json:"backup_key,omitempty"`
	CloudFrontInvID string       `json:"cloudfront_inv_id,omitempty"`
	CloudflareReady bool         `json:"cloudflare_ready"`
}

type RejectRequest struct {
	Reason string `json:"reason" binding:"required"`
}

type RejectResponse struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

type ListUploadsRequest struct {
	Status   string `form:"status"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	SortBy   string `form:"sort_by"`
	SortDir  string `form:"sort_dir"`
}

type ListUploadsResponse struct {
	Uploads    []*Upload `json:"uploads"`
	Total      int64     `json:"total"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
	TotalPages int       `json:"total_pages"`
}

type CreateUserRequest struct {
	Email string   `json:"email" binding:"required"`
	Name  string   `json:"name" binding:"required"`
	Role  UserRole `json:"role" binding:"required"`
}

type UpdateUserRequest struct {
	Name string   `json:"name"`
	Role UserRole `json:"role"`
}

type ListAuditLogsRequest struct {
	UploadID  string `form:"upload_id"`
	Action    string `form:"action"`
	UserEmail string `form:"user_email"`
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
}

type ListAuditLogsResponse struct {
	Logs       []*AuditLog `json:"logs"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

type HealthCheckResponse struct {
	Status       string            `json:"status"`
	Version      string            `json:"version"`
	Dependencies map[string]string `json:"dependencies"`
}
