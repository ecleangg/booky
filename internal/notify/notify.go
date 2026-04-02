package notify

import (
	"context"
	"time"

	"github.com/ecleangg/booky/internal/config"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

type Category string

const (
	CategoryDailyCloseFailure     Category = "daily_close_failure"
	CategoryDailyCloseWarning     Category = "daily_close_warning"
	CategoryManualReviewRequired  Category = "manual_review_required"
	CategoryWebhookIngestionError Category = "webhook_ingestion_error"
	CategoryWebhookReviewRequired Category = "webhook_review_required"
	CategorySchedulerFailure      Category = "scheduler_failure"
	CategorySchedulerConfig       Category = "scheduler_config"
	CategoryFilingDraft           Category = "filing_draft"
	CategoryFilingZeroReminder    Category = "filing_zero_reminder"
	CategoryFilingFailure         Category = "filing_failure"
)

type Attachment struct {
	Filename    string
	ContentType string
	Content     []byte
}

type Notification struct {
	Severity     Severity
	Category     Category
	PostingDate  *time.Time
	CompanyID    string
	To           []string
	Subject      string
	SummaryLines []string
	DetailLines  []string
	ActionLines  []string
	Attachments  []Attachment
}

type Notifier interface {
	Send(ctx context.Context, notification Notification) error
}

func NewNotifier(cfg config.NotificationsConfig) Notifier {
	if !cfg.Resend.Enabled {
		return nil
	}
	return NewResendNotifier(cfg.Resend)
}
