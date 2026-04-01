package filings

import (
	"strings"
	"testing"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/domain"
)

func TestRenderOSSUnionIncludesCorrections(t *testing.T) {
	cfg := testFilingsConfig()
	correctionPeriod := "2025-Q4"
	rendered, err := RenderOSSUnion(cfg, "2026-Q1", []domain.OSSUnionEntry{
		{
			OriginIdentifier:      "SE",
			ConsumptionCountry:    "DE",
			SaleType:              "SERVICES",
			VATRateBasisPoints:    1900,
			TaxableAmountEURCents: 100050,
			VATAmountEURCents:     19010,
			ReviewState:           domain.FilingReviewStateReady,
		},
		{
			OriginIdentifier:       "SE",
			ConsumptionCountry:     "DE",
			SaleType:               "SERVICES",
			VATRateBasisPoints:     1900,
			TaxableAmountEURCents:  -20000,
			VATAmountEURCents:      -3800,
			CorrectionTargetPeriod: &correctionPeriod,
			ReviewState:            domain.FilingReviewStateReady,
		},
	})
	if err != nil {
		t.Fatalf("RenderOSSUnion returned error: %v", err)
	}

	got := string(rendered.Content)
	if !strings.Contains(got, "OSS_001;\r\nSE556000016701;1;2026;\r\n") {
		t.Fatalf("unexpected OSS header:\n%s", got)
	}
	if !strings.Contains(got, "SE;DE;19,00;1000,50;190,10;SERVICES;") {
		t.Fatalf("missing OSS main row:\n%s", got)
	}
	if !strings.Contains(got, "CORRECTIONS\r\n2025Q4;DE;-38,00;") {
		t.Fatalf("missing OSS correction row:\n%s", got)
	}
}

func TestRenderPeriodicSummaryGroupsBuyerRows(t *testing.T) {
	cfg := testFilingsConfig()
	rendered, err := RenderPeriodicSummary(cfg, "2026-03", []domain.PeriodicSummaryEntry{
		{
			BuyerVATNumber:    "DE123456789",
			RowType:           "goods",
			ExportedAmountSEK: 2500,
			ReviewState:       domain.FilingReviewStateReady,
		},
		{
			BuyerVATNumber:    "DE123456789",
			RowType:           "services",
			ExportedAmountSEK: -300,
			ReviewState:       domain.FilingReviewStateReady,
		},
	})
	if err != nil {
		t.Fatalf("RenderPeriodicSummary returned error: %v", err)
	}

	got := string(rendered.Content)
	if !strings.Contains(got, "SKV574008;\r\nSE556000016701;2603;eclean Finance;+46701234567;finance@eclean.gg\r\n") {
		t.Fatalf("unexpected PS header:\n%s", got)
	}
	if !strings.Contains(got, "DE123456789;2500;;-300;") {
		t.Fatalf("missing PS buyer row:\n%s", got)
	}
}

func testFilingsConfig() config.Config {
	return config.Config{
		Filings: config.FilingsConfig{
			OSSUnion: config.OSSUnionFilingsConfig{
				IdentifierNumber: "SE556000016701",
				OriginCountry:    "SE",
			},
			PeriodicSummary: config.PeriodicSummaryFilingsConfig{
				ReportingVATNumber: "SE556000016701",
				ResponsibleName:    "eclean Finance",
				ResponsiblePhone:   "+46701234567",
				ResponsibleEmail:   "finance@eclean.gg",
			},
		},
	}
}
