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
    // Create SDK client
    client := sdk.NewClient(
        "http://localhost:30200",
        "your-auth-token",
        "user@example.com",
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
- **ApproveUpload** - Approve an upload (reviewer only)
- **RejectUpload** - Reject an upload with reason (reviewer only)
- **PurgeCloudflareCache** - Manually purge Cloudflare cache

### User Management

- **CreateUser** - Create a new user
- **GetUser** - Get user by ID
- **UpdateUser** - Update user details
- **DeleteUser** - Delete a user

### Audit & Monitoring

- **ListAuditLogs** - List audit logs with filtering
- **HealthCheck** - Check service health

## Configuration

The client requires three parameters:

- **baseURL**: Service API endpoint
- **authToken**: Bearer authentication token
- **userEmail**: Email of the user making requests

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
- User management
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

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/uploads` | Upload file |
| GET | `/api/v1/uploads` | List uploads |
| GET | `/api/v1/uploads/{id}` | Get upload status |
| POST | `/api/v1/uploads/{id}/approve` | Approve upload |
| POST | `/api/v1/uploads/{id}/reject` | Reject upload |
| POST | `/api/v1/uploads/{id}/purge-cloudflare` | Purge cache |
| POST | `/api/v1/users` | Create user |
| GET | `/api/v1/users/{id}` | Get user |
| PUT | `/api/v1/users/{id}` | Update user |
| DELETE | `/api/v1/users/{id}` | Delete user |
| GET | `/api/v1/audit-logs` | List audit logs |
| GET | `/health` | Health check |

## License

See LICENSE file in the repository root.
