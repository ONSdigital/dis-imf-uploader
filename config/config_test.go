package config

import (
	"os"
	"testing"
	"time"

	"github.com/ONSdigital/dp-authorisation/v2/authorisation"
	dpMongo "github.com/ONSdigital/dp-mongodb/v3/mongodb"
	. "github.com/smartystreets/goconvey/convey"
)

func TestConfig(t *testing.T) {
	os.Clearenv()
	cfg = nil // Reset the singleton
	var err error
	var configuration *Config

	Convey("Given an environment with no environment variables set", t, func() {
		Convey("Then cfg should be nil", func() {
			So(cfg, ShouldBeNil)
		})

		Convey("When the config values are retrieved", func() {
			Convey("Then there should be no error returned, and values are as expected", func() {
				configuration, err = Get()
				So(err, ShouldBeNil)
				So(configuration, ShouldResemble, &Config{
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
				})
			})

			Convey("And a second call to config should return the same config", func() {
				newCfg, newErr := Get()
				So(newErr, ShouldBeNil)
				So(newCfg, ShouldEqual, configuration)
			})
		})
	})
}

func TestConfigWithEnvironmentVariables(t *testing.T) {
	os.Clearenv()
	cfg = nil // Reset the singleton

	Convey("Given environment variables are set", t, func() {
		_ = os.Setenv("BIND_ADDR", "localhost:8080")
		_ = os.Setenv("MAX_UPLOAD_SIZE", "1073741824") // 1GB
		_ = os.Setenv("S3_BUCKET", "test-bucket")
		_ = os.Setenv("S3_PREFIX", "test/")
		_ = os.Setenv("SLACK_ENABLED", "true")
		_ = os.Setenv("VALIDATION_ENABLED", "false")

		Convey("When the config is retrieved", func() {
			configuration, err := Get()

			Convey("Then there should be no error and env vars should override defaults", func() {
				So(err, ShouldBeNil)
				So(configuration.BindAddr, ShouldEqual, "localhost:8080")
				So(configuration.MaxUploadSize, ShouldEqual, 1073741824)
				So(configuration.Bucket, ShouldEqual, "test-bucket")
				So(configuration.S3Config.Prefix, ShouldEqual, "test/")
				So(configuration.SlackConfig.Enabled, ShouldBeTrue)
				So(configuration.ValidationConfig.Enabled, ShouldBeFalse)
			})
		})

		Reset(func() {
			os.Clearenv()
			cfg = nil
		})
	})
}

func TestConfigCollectionNames(t *testing.T) {
	Convey("Given collection name constants", t, func() {
		Convey("Then collection titles and names should be properly defined", func() {
			So(UploadsCollectionTitle, ShouldEqual, "FileUploadsCollection")
			So(UploadsCollectionName, ShouldEqual, "uploads")
			So(BackupsCollectionTitle, ShouldEqual, "FileBackupsCollection")
			So(BackupsCollectionName, ShouldEqual, "backups")
			So(AuditLogsCollectionTitle, ShouldEqual, "FileAuditLogsCollection")
			So(AuditLogsCollectionName, ShouldEqual, "audit_logs")
		})
	})
}
