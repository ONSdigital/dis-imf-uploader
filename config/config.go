package config

import (
	"time"

	"github.com/ONSdigital/dp-authorisation/v2/authorisation"
	dpMongo "github.com/ONSdigital/dp-mongodb/v3/mongodb"
	"github.com/kelseyhightower/envconfig"
)

type MongoConfig struct {
	dpMongo.MongoDriverConfig
}

type S3Config struct {
	Bucket      string `envconfig:"S3_BUCKET"`
	Prefix      string `envconfig:"S3_PREFIX"`
	Region      string `envconfig:"AWS_REGION"`
	EndpointURL string `envconfig:"AWS_ENDPOINT_URL"`
}

type CloudFrontConfig struct {
	DistributionID string `envconfig:"CF_DIST_ID"`
}

type CloudflareConfig struct {
	Token  string `envconfig:"CLOUDFLARE_TOKEN"`
	Email  string `envconfig:"CLOUDFLARE_EMAIL"`
	ZoneID string `envconfig:"CLOUDFLARE_ZONE_ID"`
}

type AWSProfileConfig struct {
	Profile string `envconfig:"AWS_PROFILE"`
}

type SlackConfig struct {
	Enabled           bool   `envconfig:"SLACK_ENABLED"`
	WebhookURL        string `envconfig:"SLACK_WEBHOOK_URL"`
	Channel           string `envconfig:"SLACK_CHANNEL"`
	BotName           string `envconfig:"SLACK_BOT_NAME"`
	NotifyOnUpload    bool   `envconfig:"SLACK_NOTIFY_ON_UPLOAD"`
	NotifyOnApprove   bool   `envconfig:"SLACK_NOTIFY_ON_APPROVE"`
	NotifyOnReject    bool   `envconfig:"SLACK_NOTIFY_ON_REJECT"`
	NotifyOnError     bool   `envconfig:"SLACK_NOTIFY_ON_ERROR"`
	ReviewersMentions string `envconfig:"SLACK_REVIEWERS_MENTIONS"`
}

type RedisConfig struct {
	Addr     string `envconfig:"REDIS_ADDR"`
	Password string `envconfig:"REDIS_PASSWORD"`
	DB       int    `envconfig:"REDIS_DB"`
	Prefix   string `envconfig:"REDIS_PREFIX"`
}

type ValidationConfig struct {
	Enabled           bool     `envconfig:"VALIDATION_ENABLED"`
	AllowedExtensions []string `envconfig:"ALLOWED_EXTENSIONS"`
	AllowedMimeTypes  []string `envconfig:"ALLOWED_MIME_TYPES"`
}

type Config struct {
	BindAddr                   string        `envconfig:"BIND_ADDR"`
	GracefulShutdownTimeout    time.Duration `envconfig:"GRACEFUL_SHUTDOWN_TIMEOUT"`
	HealthCheckInterval        time.Duration `envconfig:"HEALTHCHECK_INTERVAL"`
	HealthCheckCriticalTimeout time.Duration `envconfig:"HEALTHCHECK_CRITICAL_TIMEOUT"`
	OTBatchTimeout             time.Duration `envconfig:"OTEL_BATCH_TIMEOUT"`
	OTExporterOTLPEndpoint     string        `envconfig:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	OTServiceName              string        `envconfig:"OTEL_SERVICE_NAME"`
	OtelEnabled                bool          `envconfig:"OTEL_ENABLED"`
	ServiceAuthToken           string        `envconfig:"SERVICE_AUTH_TOKEN"`
	MaxUploadSize              int64         `envconfig:"MAX_UPLOAD_SIZE"`
	TempStorageTimeout         time.Duration `envconfig:"TEMP_STORAGE_TIMEOUT"`
	CloudFrontCheckInterval    time.Duration `envconfig:"CF_CHECK_INTERVAL"`
	MongoConfig
	S3Config
	CloudFrontConfig
	CloudflareConfig
	AWSProfileConfig
	SlackConfig
	RedisConfig
	ValidationConfig
	AuthConfig *authorisation.Config
}

var cfg *Config

const (
	UploadsCollectionTitle   = "FileUploadsCollection"
	UploadsCollectionName    = "uploads"
	UsersCollectionTitle     = "FileUsersCollection"
	UsersCollectionName      = "users"
	BackupsCollectionTitle   = "FileBackupsCollection"
	BackupsCollectionName    = "backups"
	AuditLogsCollectionTitle = "FileAuditLogsCollection"
	AuditLogsCollectionName  = "audit_logs"
)

func Get() (*Config, error) {
	if cfg != nil {
		return cfg, nil
	}

	cfg = &Config{
		BindAddr:                   "localhost:30200",
		GracefulShutdownTimeout:    5 * time.Second,
		HealthCheckInterval:        30 * time.Second,
		HealthCheckCriticalTimeout: 90 * time.Second,
		OTBatchTimeout:             5 * time.Second,
		OTExporterOTLPEndpoint:     "localhost:4317",
		OTServiceName:              "file-upload-service",
		OtelEnabled:                false,
		MaxUploadSize:              500 * 1024 * 1024,
		TempStorageTimeout:         24 * time.Hour,
		CloudFrontCheckInterval:    10 * time.Second,
		S3Config: S3Config{
			Bucket:      "ons-dp-local-static",
			Prefix:      "imf/",
			Region:      "eu-west-2",
			EndpointURL: "",
		},
		CloudFrontConfig: CloudFrontConfig{
			DistributionID: "",
		},
		CloudflareConfig: CloudflareConfig{
			Token:  "",
			Email:  "",
			ZoneID: "",
		},
		AWSProfileConfig: AWSProfileConfig{
			Profile: "dp-sandbox",
		},
		SlackConfig: SlackConfig{
			Enabled:         false,
			BotName:         "File Upload Service",
			NotifyOnUpload:  true,
			NotifyOnApprove: true,
			NotifyOnReject:  true,
			NotifyOnError:   true,
		},
		RedisConfig: RedisConfig{
			Addr:     "",
			Password: "",
			DB:       0,
			Prefix:   "imf:temp:",
		},
		ValidationConfig: ValidationConfig{
			Enabled:           true,
			AllowedExtensions: []string{".pdf", ".xlsx", ".xls", ".csv", ".doc", ".docx"},
			AllowedMimeTypes: []string{
				"application/pdf",
				"application/vnd.ms-excel",
				"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
				"text/csv",
				"application/msword",
				"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
				"application/octet-stream",
			},
		},
		MongoConfig: MongoConfig{
			MongoDriverConfig: dpMongo.MongoDriverConfig{
				ClusterEndpoint: "localhost:27017",
				Username:        "",
				Password:        "",
				Database:        "file_uploads",
				Collections: map[string]string{
					UploadsCollectionTitle:   UploadsCollectionName,
					UsersCollectionTitle:     UsersCollectionName,
					BackupsCollectionTitle:   BackupsCollectionName,
					AuditLogsCollectionTitle: AuditLogsCollectionName,
				},
				ReplicaSet:                    "",
				IsStrongReadConcernEnabled:    false,
				IsWriteConcernMajorityEnabled: true,
				ConnectTimeout:                5 * time.Second,
				QueryTimeout:                  15 * time.Second,
				TLSConnectionConfig: dpMongo.TLSConnectionConfig{
					IsSSL: false,
				},
			},
		},
		AuthConfig: authorisation.NewDefaultConfig(),
	}

	if err := envconfig.Process("", cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
