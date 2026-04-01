package filings

import (
	"testing"
	"time"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/domain"
)

func TestFilingFirstSendAtUsesLeadTimeAndLocalClock(t *testing.T) {
	cfg := config.Config{
		App: config.AppConfig{Timezone: "Europe/Stockholm"},
		Filings: config.FilingsConfig{
			LeadTimeDays:  7,
			SendTimeLocal: "09:00",
		},
	}

	got, err := filingFirstSendAt(domain.FilingKindOSSUnion, "2026-Q1", cfg)
	if err != nil {
		t.Fatalf("filingFirstSendAt returned error: %v", err)
	}

	want := time.Date(2026, time.April, 23, 9, 0, 0, 0, got.Location())
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want.Format(time.RFC3339), got.Format(time.RFC3339))
	}
}

func TestFilingDeadlineForPeriodicSummary(t *testing.T) {
	loc := time.UTC
	got, err := filingDeadline(domain.FilingKindPeriodicSummary, "2026-03", loc)
	if err != nil {
		t.Fatalf("filingDeadline returned error: %v", err)
	}

	want := time.Date(2026, time.April, 25, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want.Format("2006-01-02"), got.Format("2006-01-02"))
	}
}
