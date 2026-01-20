# DIS IMF Uploader Service

A secure file upload service for IMF files with approval workflow, S3 storage, CloudFront invalidation, and Cloudflare cache purging.

## Features

### Core Functionality
- ✅ **File Upload with Review Process**: Upload files that require approval before publishing
- ✅ **Role-Based Access Control**: Uploader, Reviewer, and Admin roles
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
- `POST /api/v1/uploads` - Upload file for review
- `GET /api/v1/uploads` - List uploads with pagination and filtering
- `GET /api/v1/uploads/{id}` - Get upload status
- `POST /api/v1/uploads/{id}/approve` - Approve upload (reviewer only)
- `POST /api/v1/uploads/{id}/reject` - Reject upload (reviewer only)
- `POST /api/v1/uploads/{id}/purge-cloudflare` - Manually purge Cloudflare cache

#### User Management
- `POST /api/v1/users` - Create user
- `GET /api/v1/users/{id}` - Get user
- `PUT /api/v1/users/{id}` - Update user
- `DELETE /api/v1/users/{id}` - Delete user

#### Audit & Health
- `GET /api/v1/audit-logs` - List audit logs with filtering
- `GET /health` - Service health check

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

client := sdk.NewClient("http://localhost:30200", "token", "user@example.com")
upload, err := client.UploadFile(ctx, "file.pdf", fileData)
```

See [SDK Documentation](./sdk/README.md) for complete examples.

## Configuration

Environment variables:

```bash
# Server
BIND_ADDR="localhost:30200"
MAX_UPLOAD_SIZE="524288000"  # 500MB

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
├── api/          # HTTP handlers, middleware, routes
├── config/       # Configuration
├── models/       # Data models
├── mongo/        # MongoDB operations
├── notifications/# Slack notifications
├── storage/      # S3, CloudFront, Cloudflare clients
├── temp/         # Temporary file storage
├── sdk/          # Go SDK
└── main.go       # Entry point with graceful shutdown
```

## Security

- Bearer token authentication
- Role-based access control (Uploader, Reviewer, Admin)
- File size limits and checksum verification
- Complete audit trail
- Graceful shutdown with proper cleanup

## Development

```bash
# Run tests
go test ./... -v

# Build for production
CGO_ENABLED=0 go build -ldflags="-s -w" -o imf-uploader
```

## License

Copyright © 2026 ONS Digital
