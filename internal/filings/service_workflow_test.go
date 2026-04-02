package filings

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/notify"
	"github.com/ecleangg/booky/internal/testutil"
	"github.com/google/uuid"
)

func TestRunPeriodSendsZeroReminderForEmptyOSSPeriod(t *testing.T) {
	repo, _ := testutil.NewTestRepository(t)
	notifier := &testutil.RecordingNotifier{}
	service := NewService(testutil.TestConfig(), repo, notifier, nil, testLogger())

	export, err := service.RunPeriod(context.Background(), domain.FilingKindOSSUnion, "2026-Q1")
	if err != nil {
		t.Fatalf("RunPeriod returned error: %v", err)
	}
	if export == nil || export.Checksum != "zero-reminder" {
		t.Fatalf("unexpected export %#v", export)
	}

	notifications := notifier.Snapshot()
	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}
	if notifications[0].Category != notify.CategoryFilingZeroReminder {
		t.Fatalf("unexpected notification category %q", notifications[0].Category)
	}

	period, err := repo.Queries().GetFilingPeriod(context.Background(), testutil.TestConfig().Bokio.CompanyID, domain.FilingKindOSSUnion, "2026-Q1")
	if err != nil {
		t.Fatalf("GetFilingPeriod returned error: %v", err)
	}
	if period.LastEvaluationStatus != domain.FilingPeriodStatusNoDataReminder {
		t.Fatalf("unexpected filing status %q", period.LastEvaluationStatus)
	}
	if period.ZeroReminderSentAt == nil {
		t.Fatal("expected zero reminder timestamp")
	}
}

func TestRunPeriodStoresRenderedExportAndSkipsUnchangedRevision(t *testing.T) {
	repo, _ := testutil.NewTestRepository(t)
	notifier := &testutil.RecordingNotifier{}
	service := NewService(testutil.TestConfig(), repo, notifier, nil, testLogger())
	companyID := testutil.TestConfig().Bokio.CompanyID

	entry := domain.OSSUnionEntry{
		ID:                    uuid.New(),
		BokioCompanyID:        companyID,
		SourceGroupID:         "charge:ch_123:sale",
		SourceObjectType:      "charge",
		SourceObjectID:        "ch_123",
		OriginalSupplyPeriod:  "2026-Q1",
		FilingPeriod:          "2026-Q1",
		ConsumptionCountry:    "DE",
		OriginCountry:         "SE",
		OriginIdentifier:      "SE556000016701",
		SaleType:              "SERVICES",
		VATRateBasisPoints:    1900,
		TaxableAmountEURCents: 10000,
		VATAmountEURCents:     1900,
		ReviewState:           domain.FilingReviewStateReady,
		Payload:               []byte(`{"entry":"oss"}`),
	}
	if err := repo.Queries().UpsertOSSUnionEntries(context.Background(), []domain.OSSUnionEntry{entry}); err != nil {
		t.Fatalf("UpsertOSSUnionEntries returned error: %v", err)
	}

	export, err := service.RunPeriod(context.Background(), domain.FilingKindOSSUnion, "2026-Q1")
	if err != nil {
		t.Fatalf("RunPeriod returned error: %v", err)
	}
	if export == nil || export.Filename == nil {
		t.Fatalf("expected rendered export, got %#v", export)
	}

	notifications := notifier.Snapshot()
	if len(notifications) != 1 {
		t.Fatalf("expected 1 draft notification, got %d", len(notifications))
	}
	if notifications[0].Category != notify.CategoryFilingDraft {
		t.Fatalf("unexpected notification category %q", notifications[0].Category)
	}
	if len(notifications[0].Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(notifications[0].Attachments))
	}

	second, err := service.RunPeriod(context.Background(), domain.FilingKindOSSUnion, "2026-Q1")
	if err != nil {
		t.Fatalf("second RunPeriod returned error: %v", err)
	}
	if second != nil {
		t.Fatalf("expected unchanged export to return nil, got %#v", second)
	}

	exports, err := repo.Queries().ListFilingExports(context.Background(), companyID, domain.FilingKindOSSUnion, "2026-Q1")
	if err != nil {
		t.Fatalf("ListFilingExports returned error: %v", err)
	}
	if len(exports) != 1 {
		t.Fatalf("expected 1 stored export, got %d", len(exports))
	}
	if len(notifier.Snapshot()) != 1 {
		t.Fatalf("expected unchanged run to avoid extra notifications, got %d", len(notifier.Snapshot()))
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
