package accounting

import (
	"context"
	"testing"
	"time"

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

func TestNotifyFailureUsesManualReviewCategoryForReviewBlocks(t *testing.T) {
	notifier := &recordingNotifier{}
	service := &Service{
		Config: config.Config{
			Bokio:    config.BokioConfig{CompanyID: uuid.New()},
			Accounts: config.AccountsConfig{FallbackOBS: 2999},
		},
		Notify: notifier,
	}
	postingDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	reason := "missing VAT evidence"
	facts := []domain.AccountingFact{{
		SourceGroupID: "charge:ch_123:sale",
		Status:        domain.FactStatusNeedsReview,
		ReviewReason:  &reason,
		BokioAccount:  2999,
	}}

	service.notifyFailure(context.Background(), postingDate, errString("posting blocked because 1 source group(s) need review and posting.auto_post_unknown_to_obs is false"), facts)

	if len(notifier.notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifier.notifications))
	}
	got := notifier.notifications[0]
	if got.Category != notify.CategoryManualReviewRequired {
		t.Fatalf("expected manual review category, got %q", got.Category)
	}
	if got.PostingDate == nil || !got.PostingDate.Equal(postingDate) {
		t.Fatalf("expected posting date %s, got %#v", postingDate, got.PostingDate)
	}
	if len(got.ActionLines) == 0 {
		t.Fatalf("expected action lines for manual review notification")
	}
}

func TestComplianceConcernLinesFlagsOBSReviewAndRounding(t *testing.T) {
	reason := "missing customer VAT ID"
	rounding := &domain.AccountingFact{AmountSEKOre: 1}
	lines := complianceConcernLines([]domain.AccountingFact{{
		SourceGroupID: "charge:ch_123:sale",
		BokioAccount:  2999,
		Status:        domain.FactStatusNeedsReview,
		ReviewReason:  &reason,
	}}, rounding, 2999)

	if len(lines) < 3 {
		t.Fatalf("expected at least 3 compliance concern lines, got %v", lines)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
