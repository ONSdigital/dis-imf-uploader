# IMF File Upload Service SDK

Go SDK for the IMF File Upload Service API.

## Installation

```bash
go get github.com/ONSdigital/dis-imf-uploader/sdk
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/ONSdigital/dis-imf-uploader/sdk"
)

func main() {
    // Create SDK client with JWT token
    client := sdk.NewClient(
        "http://localhost:30200",
        "your-jwt-token",           // JWT token from authentication service
        "user@example.com",          // User identifier (for logging/tracking)
    )

    ctx := context.Background()

    // Upload a file
    fileData, _ := os.ReadFile("document.pdf")
    uploadResp, err := client.UploadFile(ctx, "document.pdf", fileData)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Upload ID: %s, Status: %s", uploadResp.ID, uploadResp.Status)
}
```

## Features

### File Upload Operations

- **UploadFile** - Upload a file for review
- **GetUploadStatus** - Get current status of an upload
- **ListUploads** - List uploads with filtering and pagination
- **ApproveUpload** - Approve an upload (requires `imf:approve` permission)
- **RejectUpload** - Reject an upload with reason (requires `imf:reject` permission)
- **PurgeCloudflareCache** - Manually purge Cloudflare cache (requires `imf:purge` permission)

### Audit & Monitoring

- **ListAuditLogs** - List audit logs with filtering
- **HealthCheck** - Check service health

## Authentication & Authorization

The service uses **JWT-based authentication** via the Authorization header. User authentication and authorization are handled by:

- **Authentication**: JWT tokens validated by the service
- **Authorization**: Permissions checked via `dp-permissions-api`

Required permissions for operations:
- `imf:upload` - Upload files
- `imf:read` - View uploads and audit logs
- `imf:approve` - Approve uploads
- `imf:reject` - Reject uploads
- `imf:purge` - Purge Cloudflare cache

## Configuration

The client requires three parameters:

- **baseURL**: Service API endpoint
- **authToken**: JWT Bearer authentication token
- **userEmail**: Email/identifier of the user making requests (used for audit logging)

## Custom HTTP Client

You can provide a custom HTTP client:

```go
import "net/http"

httpClient := &http.Client{
    Timeout: 10 * time.Minute,
}

client := sdk.NewClient(baseURL, token, email).WithHTTPClient(httpClient)
```

## Examples

See `example_test.go` for complete examples of:

- Upload workflow (upload → review → approve/reject)
- Audit log retrieval
- Health checks

## Error Handling

All methods return errors that include HTTP status codes and error messages from the API:

```go
upload, err := client.GetUploadStatus(ctx, "invalid-id")
if err != nil {
    // Error format: "request failed: upload not found (code: 404)"
    log.Printf("Error: %v", err)
}
```

## API Endpoints

| Method | Endpoint | Description | Required Permission |
|--------|----------|-------------|---------------------|
| POST | `/api/v1/uploads` | Upload file | `imf:upload` |
| GET | `/api/v1/uploads` | List uploads | `imf:read` |
| GET | `/api/v1/uploads/{id}` | Get upload status | `imf:read` |
| POST | `/api/v1/uploads/{id}/approve` | Approve upload | `imf:approve` |
| POST | `/api/v1/uploads/{id}/reject` | Reject upload | `imf:reject` |
| POST | `/api/v1/uploads/{id}/purge-cloudflare` | Purge cache | `imf:purge` |
| GET | `/api/v1/audit-logs` | List audit logs | `imf:read` |
| GET | `/health` | Health check | None |

## License

See LICENSE file in the repository root.
