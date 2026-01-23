# DIS IMF Uploader Service

A secure file upload service for IMF files with approval workflow, S3 storage, CloudFront invalidation, and Cloudflare cache purging.

## Features

### Core Functionality
- ✅ **File Upload with Review Process**: Upload files that require approval before publishing 
- ✅ **S3 Storage**: Direct upload to AWS S3 with automatic backup of existing files
- ✅ **CloudFront Integration**: Automatic cache invalidation on approval
- ✅ **Cloudflare Purging**: Automatic Cloudflare cache clearing on approval
- ✅ **Slack Notifications**: Real-time notifications for uploads, approvals, rejections, and errors
- ✅ **Audit Logging**: Complete audit trail of all operations
- ✅ **Temporary Storage**: Files stored temporarily until approved/rejected with TTL
- ✅ **Health Checks**: Comprehensive health monitoring of all dependencies
- ✅ **Go SDK**: Full-featured SDK for easy integration

### API Endpoints

#### Uploads
- `POST /api/v1/uploads` - Upload file for review (requires `imf:upload` permission)
- `GET /api/v1/uploads` - List uploads with pagination and filtering (requires `imf:read` permission)
- `GET /api/v1/uploads/{id}` - Get upload status (requires `imf:read` permission)
- `POST /api/v1/uploads/{id}/approve` - Approve upload (requires `imf:approve` permission)
- `POST /api/v1/uploads/{id}/reject` - Reject upload (requires `imf:reject` permission)
- `POST /api/v1/uploads/{id}/purge-cloudflare` - Manually purge Cloudflare cache (requires `imf:purge` permission)

#### Audit & Health
- `GET /api/v1/audit-logs` - List audit logs with filtering (requires `imf:read` permission)
- `GET /health` - Service health check (no authentication required)

## Quick Start

```bash
# Install dependencies
go mod download

# Build
go build -o imf-uploader

# Run with environment variables
export MONGODB_CLUSTER_ENDPOINT="localhost:27017"
export S3_BUCKET="your-bucket"
export CF_DIST_ID="your-distribution-id"
./imf-uploader
```

## SDK Usage

```bash
go get github.com/ONSdigital/dis-imf-uploader/sdk
```

```go
import "github.com/ONSdigital/dis-imf-uploader/sdk"

client := sdk.NewClient("http://localhost:30200", "your-jwt-token")
upload, err := client.UploadFile(ctx, "file.pdf", fileData)
```

See [SDK Documentation](./sdk/README.md) for complete examples.

## Configuration

Environment variables:

```bash
# Server
BIND_ADDR="localhost:30200"
MAX_UPLOAD_SIZE="524288000"  # 500MB

# Authentication
ENABLE_PERMISSIONS_AUTH="true"
ZEBEDEE_URL="http://localhost:8082"

# MongoDB
MONGODB_CLUSTER_ENDPOINT="localhost:27017"
MONGODB_DATABASE="file_uploads"

# AWS
S3_BUCKET="your-bucket"
S3_PREFIX="imf/"
AWS_REGION="eu-west-2"
CF_DIST_ID="E1234567890ABC"

# Cloudflare (optional)
CLOUDFLARE_TOKEN="your-token"
CLOUDFLARE_ZONE_ID="your-zone-id"

# Slack (optional)
SLACK_ENABLED="true"
SLACK_WEBHOOK_URL="https://hooks.slack.com/..."
```

## Workflow

1. **Upload**: User uploads file → stored temporarily, MongoDB record created, reviewers notified
2. **Review**: Reviewer approves/rejects
   - **Approve**: Backup existing → Upload to S3 → Invalidate CloudFront → Purge Cloudflare → Notify
   - **Reject**: Delete temp file → Log reason → Notify
3. **Monitor**: Audit logs, health checks, CloudFront status tracking

## Project Structure

```
├── api/          # HTTP handlers and API setup with permission-based routes
├── config/       # Configuration management
├── models/       # Data models (Upload, AuditLog)
├── mongo/        # MongoDB operations
├── notifications/# Slack notifications
├── storage/      # S3, CloudFront, Cloudflare clients
├── temp/         # Temporary file storage
├── validation/   # File validation
├── sdk/          # Go SDK for client integration
└── main.go       # Entry point with graceful shutdown
```

## Authentication & Authorization

- **JWT-based Authentication**: All API requests require a valid JWT token in the `Authorization` header
- **Permission-based Authorization**: Integration with [dp-permissions-api](https://github.com/ONSdigital/dp-permissions-api) for fine-grained access control
- **Required Permissions**:
  - `imf:upload` - Upload files
  - `imf:read` - View uploads and audit logs
  - `imf:approve` - Approve pending uploads
  - `imf:reject` - Reject pending uploads
  - `imf:purge` - Manually purge Cloudflare cache

## Security

- JWT token authentication via dp-permissions-api
- Permission-based access control for all operations
- File size limits and checksum verification
- Complete audit trail with user tracking
- Graceful shutdown with proper cleanup

## Development

```bash
# Run tests (requires MongoDB running locally)
make test

# Run tests without MongoDB integration tests
go test -short ./...

# Run tests with coverage
make convey

# Lint code
make lint

# Security audit
make audit

# Build for production
make build
```

**Note**: MongoDB integration tests require a running MongoDB instance on `localhost:27017`. Use `go test -short ./...` to skip these tests if MongoDB is not available.

## License

Copyright © 2026 ONS Digital
