package stripe

import (
	"context"
	"testing"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/notify"
	"github.com/google/uuid"
)

type recordingNotifier struct {
	notifications []notify.Notification
}

func (n *recordingNotifier) Send(_ context.Context, notification notify.Notification) error {
	n.notifications = append(n.notifications, notification)
	return nil
}

func TestNotifyWebhookFailureSendsComplianceAlert(t *testing.T) {
	notifier := &recordingNotifier{}
	service := &Service{
		Config: config.Config{Bokio: config.BokioConfig{CompanyID: uuid.New()}},
		Notify: notifier,
	}

	service.notifyWebhookFailure(context.Background(), Event{ID: "evt_123", Type: "charge.succeeded"}, errString("boom"))

	if len(notifier.notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifier.notifications))
	}
	got := notifier.notifications[0]
	if got.Category != notify.CategoryWebhookIngestionError {
		t.Fatalf("expected webhook ingestion category, got %q", got.Category)
	}
	if len(got.ActionLines) == 0 {
		t.Fatalf("expected action lines in webhook failure notification")
	}
}

func TestWebhookReviewLinesIncludesNeedsReviewReasons(t *testing.T) {
	reason := "missing VAT evidence"
	lines := webhookReviewLines([]domain.AccountingFact{{
		SourceGroupID: "charge:ch_123:sale",
		FactType:      "sale_review_obs",
		Status:        domain.FactStatusNeedsReview,
		ReviewReason:  &reason,
	}})

	if len(lines) != 1 {
		t.Fatalf("expected 1 review line, got %v", lines)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
