package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ONSdigital/dis-imf-uploader/config"
	"github.com/ONSdigital/dis-imf-uploader/models"
)

// SlackNotifier sends notifications to Slack via webhook.
type SlackNotifier struct {
	config *config.SlackConfig
}

// NotificationEvent represents an event that triggers a notification.
type NotificationEvent struct {
	Type       string
	Upload     *models.Upload
	ReviewedBy string
	Reason     string
	Error      string
}

// NewSlackNotifier creates a new SlackNotifier instance.
func NewSlackNotifier(cfg *config.SlackConfig) *SlackNotifier {
	return &SlackNotifier{config: cfg}
}

// Notify sends a notification based on the event type.
func (s *SlackNotifier) Notify(ctx context.Context, event *NotificationEvent) error {
	if !s.config.Enabled {
		return nil
	}

	switch event.Type {
	case "upload":
		if !s.config.NotifyOnUpload {
			return nil
		}
		return s.notifyUpload(ctx, event)
	case "approve":
		if !s.config.NotifyOnApprove {
			return nil
		}
		return s.notifyApprove(ctx, event)
	case "reject":
		if !s.config.NotifyOnReject {
			return nil
		}
		return s.notifyReject(ctx, event)
	case "error":
		if !s.config.NotifyOnError {
			return nil
		}
		return s.notifyError(ctx, event)
	default:
		return fmt.Errorf("unknown notification type: %s", event.Type)
	}
}

func (s *SlackNotifier) notifyUpload(ctx context.Context, event *NotificationEvent) error {
	mentions := s.getMentions()

	message := &SlackMessage{
		Username: s.config.BotName,
		Channel:  s.config.Channel,
		Text:     fmt.Sprintf("%s New file upload pending review", mentions),
		Attachments: []Attachment{
			{
				Color:     "#3366FF",
				Title:     "📤 New File Upload Pending Review",
				TitleLink: s.getReviewDashboardURL(event.Upload.ID),
				Fields: []Field{
					{
						Title: "File Name",
						Value: event.Upload.FileName,
						Short: true,
					},
					{
						Title: "File Size",
						Value: formatBytes(event.Upload.FileSize),
						Short: true,
					},
					{
						Title: "Uploaded By",
						Value: event.Upload.UploadedBy,
						Short: true,
					},
				},
				Footer:    "File Upload Service",
				Timestamp: time.Now().Unix(),
			},
		},
	}

	return s.send(ctx, message)
}

func (s *SlackNotifier) notifyApprove(ctx context.Context, event *NotificationEvent) error {
	message := &SlackMessage{
		Username: s.config.BotName,
		Channel:  s.config.Channel,
		Attachments: []Attachment{
			{
				Color:     "#36a64f",
				Title:     "✅ File Upload Approved",
				TitleLink: s.getUploadDetailsURL(event.Upload.ID),
				Fields: []Field{
					{
						Title: "File Name",
						Value: event.Upload.FileName,
						Short: true,
					},
					{
						Title: "Approved By",
						Value: event.ReviewedBy,
						Short: true,
					},
					{
						Title: "S3 Key",
						Value: fmt.Sprintf("`%s`", event.Upload.S3Key),
						Short: false,
					},
				},
				Footer:    "File Upload Service",
				Timestamp: time.Now().Unix(),
			},
		},
	}

	return s.send(ctx, message)
}

func (s *SlackNotifier) notifyReject(ctx context.Context, event *NotificationEvent) error {
	message := &SlackMessage{
		Username: s.config.BotName,
		Channel:  s.config.Channel,
		Attachments: []Attachment{
			{
				Color: "#FF6B6B",
				Title: "❌ File Upload Rejected",
				Fields: []Field{
					{
						Title: "File Name",
						Value: event.Upload.FileName,
						Short: true,
					},
					{
						Title: "Rejected By",
						Value: event.ReviewedBy,
						Short: true,
					},
					{
						Title: "Reason",
						Value: event.Reason,
						Short: false,
					},
				},
				Footer:    "File Upload Service",
				Timestamp: time.Now().Unix(),
			},
		},
	}

	return s.send(ctx, message)
}

func (s *SlackNotifier) notifyError(ctx context.Context, event *NotificationEvent) error {
	message := &SlackMessage{
		Username: s.config.BotName,
		Channel:  s.config.Channel,
		Attachments: []Attachment{
			{
				Color: "#FF0000",
				Title: "🚨 Upload Processing Error",
				Fields: []Field{
					{
						Title: "File Name",
						Value: event.Upload.FileName,
						Short: true,
					},
					{
						Title: "Error",
						Value: fmt.Sprintf("`%s`", event.Error),
						Short: false,
					},
				},
				Footer:    "File Upload Service",
				Timestamp: time.Now().Unix(),
			},
		},
	}

	return s.send(ctx, message)
}

func (s *SlackNotifier) send(ctx context.Context, message *SlackMessage) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.config.WebhookURL,
		bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned %d", resp.StatusCode)
	}

	return nil
}

func (s *SlackNotifier) getMentions() string {
	if s.config.ReviewersMentions == "" {
		return ""
	}
	mentions := strings.Split(s.config.ReviewersMentions, ",")
	userMentions := make([]string, 0)
	for _, mention := range mentions {
		trimmed := strings.TrimSpace(mention)
		if trimmed != "" {
			userMentions = append(userMentions, fmt.Sprintf("<@%s>", trimmed))
		}
	}
	return strings.Join(userMentions, " ")
}

func (s *SlackNotifier) getReviewDashboardURL(uploadID string) string {
	return fmt.Sprintf("https://your-app.example.com/dashboard/review/%s", uploadID)
}

func (s *SlackNotifier) getUploadDetailsURL(uploadID string) string {
	return fmt.Sprintf("https://your-app.example.com/dashboard/uploads/%s", uploadID)
}

func formatBytes(numBytes int64) string {
	const unit = 1024
	if numBytes < unit {
		return fmt.Sprintf("%d B", numBytes)
	}
	div, exp := int64(unit), 0
	for n := numBytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(numBytes)/float64(div), "KMGTPE"[exp])
}

// SlackMessage represents a Slack webhook message payload.
type SlackMessage struct {
	Username    string       `json:"username"`
	Channel     string       `json:"channel"`
	Text        string       `json:"text,omitempty"`
	Attachments []Attachment `json:"attachments"`
}

// Attachment represents a Slack message attachment.
type Attachment struct {
	Color     string  `json:"color"`
	Title     string  `json:"title"`
	TitleLink string  `json:"title_link,omitempty"`
	Fields    []Field `json:"fields"`
	Footer    string  `json:"footer"`
	Timestamp int64   `json:"ts"`
}

// Field represents a field in a Slack attachment.
type Field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}
