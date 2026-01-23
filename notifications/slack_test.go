package notifications

import (
	"context"
	"testing"

	"github.com/ONSdigital/dis-imf-uploader/config"
	"github.com/ONSdigital/dis-imf-uploader/models"
	"github.com/google/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSlackNotifier(t *testing.T) {
	Convey("Given a Slack notifier", t, func() {
		cfg := &config.SlackConfig{
			Enabled:         true,
			WebhookURL:      "https://hooks.slack.com/services/TEST/WEBHOOK",
			Channel:         "#uploads",
			BotName:         "Test Bot",
			NotifyOnUpload:  true,
			NotifyOnApprove: true,
			NotifyOnReject:  true,
			NotifyOnError:   true,
		}

		notifier := NewSlackNotifier(cfg)

		Convey("When creating upload notification event", func() {
			upload := &models.Upload{
				ID:         uuid.New().String(),
				FileName:   "test.pdf",
				FileSize:   1024,
				UploadedBy: "user@example.com",
				Status:     models.StatusPending,
			}

			event := &NotificationEvent{
				Type:   "upload",
				Upload: upload,
			}

			_ = notifier

			Convey("Then the notification should be created", func() {
				So(event.Type, ShouldEqual, "upload")
				So(event.Upload.FileName, ShouldEqual, "test.pdf")
			})
		})

		Convey("When notifying with disabled notifications", func() {
			disabledCfg := &config.SlackConfig{
				Enabled: false,
			}
			disabledNotifier := NewSlackNotifier(disabledCfg)

			upload := &models.Upload{
				ID:       uuid.New().String(),
				FileName: "test.pdf",
			}

			event := &NotificationEvent{
				Type:   "upload",
				Upload: upload,
			}

			err := disabledNotifier.Notify(context.Background(), event)

			Convey("Then no notification should be sent", func() {
				So(err, ShouldBeNil)
			})
		})

		Convey("When formatting bytes", func() {
			Convey("Then small bytes should format correctly", func() {
				result := formatBytes(512)
				So(result, ShouldEqual, "512 B")
			})

			Convey("And kilobytes should format correctly", func() {
				result := formatBytes(1024 * 512)
				So(result, ShouldContainSubstring, "KB")
			})

			Convey("And megabytes should format correctly", func() {
				result := formatBytes(1024 * 1024 * 5)
				So(result, ShouldContainSubstring, "MB")
			})
		})

		Convey("When getting mentions", func() {
			cfgWithMentions := &config.SlackConfig{
				ReviewersMentions: "U1234567890,U0987654321",
			}
			notifierWithMentions := NewSlackNotifier(cfgWithMentions)

			mentions := notifierWithMentions.getMentions()

			Convey("Then mentions should be formatted correctly", func() {
				So(mentions, ShouldContainSubstring, "<@U1234567890>")
				So(mentions, ShouldContainSubstring, "<@U0987654321>")
			})
		})
	})
}
