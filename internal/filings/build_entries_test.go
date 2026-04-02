package filings

import (
	"context"
	"testing"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/testutil"
)

func TestBuildOSSUnionEntryUsesConfiguredVATRate(t *testing.T) {
	service := &Service{Config: testutil.TestConfig()}

	entry, err := service.buildOSSUnionEntry(context.Background(), filingContext{
		PostingDate:        time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC),
		OriginalSupplyDate: time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC),
		Country:            "DE",
		SaleType:           "SERVICES",
		RevenueSEKOre:      23084,
		VATSEKOre:          342,
	})
	if err != nil {
		t.Fatalf("buildOSSUnionEntry returned error: %v", err)
	}
	if entry == nil {
		t.Fatal("expected entry")
	}
	if entry.ReviewState != domain.FilingReviewStateReady {
		t.Fatalf("expected ready entry, got %q", entry.ReviewState)
	}
	if entry.VATRateBasisPoints != 1900 {
		t.Fatalf("expected configured VAT rate 1900, got %d", entry.VATRateBasisPoints)
	}
}
