package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/ONSdigital/dis-imf-uploader/models"
)

// Client is the SDK client for the file upload service
type Client struct {
	baseURL    string
	authToken  string
	userEmail  string
	httpClient *http.Client
}

// NewClient creates a new SDK client
func NewClient(baseURL, authToken, userEmail string) *Client {
	return &Client{
		baseURL:   baseURL,
		authToken: authToken,
		userEmail: userEmail,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// WithHTTPClient allows setting a custom HTTP client
func (c *Client) WithHTTPClient(client *http.Client) *Client {
	c.httpClient = client
	return c
}

// UploadFile uploads a file for review
func (c *Client) UploadFile(ctx context.Context, fileName string, fileData []byte) (*models.UploadResponse, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := part.Write(fileData); err != nil {
		return nil, fmt.Errorf("failed to write file data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/uploads", body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.authToken)
	req.Header.Set("X-User-Email", c.userEmail)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return nil, c.parseError(resp)
	}

	var uploadResp models.UploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &uploadResp, nil
}

// GetUploadStatus retrieves the status of an upload
func (c *Client) GetUploadStatus(ctx context.Context, uploadID string) (*models.Upload, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		c.baseURL+"/api/v1/uploads/"+uploadID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var upload models.Upload
	if err := json.NewDecoder(resp.Body).Decode(&upload); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &upload, nil
}

// ListUploads lists uploads with optional filtering
func (c *Client) ListUploads(ctx context.Context, status string, page, pageSize int) (*models.ListUploadsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/uploads?page=%d&page_size=%d", c.baseURL, page, pageSize)
	if status != "" {
		url += "&status=" + status
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var listResp models.ListUploadsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &listResp, nil
}

// ApproveUpload approves an upload for publishing
func (c *Client) ApproveUpload(ctx context.Context, uploadID string) (*models.ApproveResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/api/v1/uploads/"+uploadID+"/approve", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var approveResp models.ApproveResponse
	if err := json.NewDecoder(resp.Body).Decode(&approveResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &approveResp, nil
}

// RejectUpload rejects an upload
func (c *Client) RejectUpload(ctx context.Context, uploadID, reason string) error {
	reqBody := models.RejectRequest{Reason: reason}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/api/v1/uploads/"+uploadID+"/reject", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}

	return nil
}

// PurgeCloudflareCache manually purges Cloudflare cache for an upload
func (c *Client) PurgeCloudflareCache(ctx context.Context, uploadID string) error {
	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/api/v1/uploads/"+uploadID+"/purge-cloudflare", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}

	return nil
}

// CreateUser creates a new user
func (c *Client) CreateUser(ctx context.Context, email, name string, role models.UserRole) (*models.User, error) {
	reqBody := models.CreateUserRequest{
		Email: email,
		Name:  name,
		Role:  role,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/api/v1/users", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, c.parseError(resp)
	}

	var user models.User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &user, nil
}

// GetUser retrieves a user by ID
func (c *Client) GetUser(ctx context.Context, userID string) (*models.User, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		c.baseURL+"/api/v1/users/"+userID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var user models.User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &user, nil
}

// UpdateUser updates a user
func (c *Client) UpdateUser(ctx context.Context, userID, name string, role models.UserRole) error {
	reqBody := models.UpdateUserRequest{
		Name: name,
		Role: role,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT",
		c.baseURL+"/api/v1/users/"+userID, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}

	return nil
}

// DeleteUser deletes a user
func (c *Client) DeleteUser(ctx context.Context, userID string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE",
		c.baseURL+"/api/v1/users/"+userID, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}

	return nil
}

// ListAuditLogs lists audit logs with optional filtering
func (c *Client) ListAuditLogs(ctx context.Context, uploadID, action, userEmail string, page, pageSize int) (*models.ListAuditLogsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/audit-logs?page=%d&page_size=%d", c.baseURL, page, pageSize)
	if uploadID != "" {
		url += "&upload_id=" + uploadID
	}
	if action != "" {
		url += "&action=" + action
	}
	if userEmail != "" {
		url += "&user_email=" + userEmail
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var listResp models.ListAuditLogsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &listResp, nil
}

// HealthCheck checks the health of the service
func (c *Client) HealthCheck(ctx context.Context) (*models.HealthCheckResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var healthResp models.HealthCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &healthResp, nil
}

// setHeaders sets common headers for requests
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.authToken)
	req.Header.Set("X-User-Email", c.userEmail)
}

// parseError parses an error response
func (c *Client) parseError(resp *http.Response) error {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	var errResp models.ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return fmt.Errorf("request failed: %s (code: %d)", errResp.Error, errResp.Code)
}
