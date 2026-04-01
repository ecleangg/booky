package accounting

import (
	"testing"
	"time"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/domain"
	"github.com/google/uuid"
)

func TestAggregateJournalItemsAddsRoundingWithinTolerance(t *testing.T) {
	postingDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	facts := []domain.AccountingFact{
		{ID: uuid.New(), PostingDate: postingDate, BokioAccount: 1580, Direction: domain.DirectionDebit, AmountSEKOre: 10000},
		{ID: uuid.New(), PostingDate: postingDate, BokioAccount: 3001, Direction: domain.DirectionCredit, AmountSEKOre: 9999},
	}

	items, roundingFact, summary, err := AggregateJournalItems(facts, 3740, 2)
	if err != nil {
		t.Fatalf("AggregateJournalItems returned error: %v", err)
	}
	if roundingFact == nil {
		t.Fatalf("expected rounding fact to be created")
	}
	if roundingFact.BokioAccount != 3740 {
		t.Fatalf("expected rounding account 3740, got %d", roundingFact.BokioAccount)
	}
	if got := summary["rounding_amount_ore"]; got != int64(1) {
		t.Fatalf("expected rounding amount 1 ore, got %#v", got)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 journal items, got %d", len(items))
	}
}

func TestResolveSaleUsesConfiguredVATFallback(t *testing.T) {
	cfg := testConfig()
	result := ResolveSale(cfg, SaleClassificationInput{
		Country:     "SE",
		IsB2B:       false,
		GrossSEKOre: 12500,
		Evidence: SaleEvidence{
			CountryEvidence: true,
			VATMode:         "domestic_se",
		},
	})

	if result.ReviewReason != "" {
		t.Fatalf("expected no review reason, got %q", result.ReviewReason)
	}
	if result.RevenueSEKOre != 10000 {
		t.Fatalf("expected revenue 10000 ore, got %d", result.RevenueSEKOre)
	}
	if result.VATSEKOre != 2500 {
		t.Fatalf("expected vat 2500 ore, got %d", result.VATSEKOre)
	}
	if result.RevenueAccount != 3001 || result.OutputVATAccount != 2611 {
		t.Fatalf("unexpected account mapping: %#v", result)
	}
}

func TestResolveSaleRequiresEUB2BEvidence(t *testing.T) {
	cfg := testConfig()
	result := ResolveSale(cfg, SaleClassificationInput{
		Country:     "DE",
		IsB2B:       true,
		GrossSEKOre: 12500,
		Evidence: SaleEvidence{
			CountryEvidence: true,
		},
	})

	if result.ReviewReason == "" {
		t.Fatalf("expected review reason for unsupported EU B2B evidence")
	}
}

func TestResolveSaleAllowsStripeTaxBackedEUB2B(t *testing.T) {
	cfg := testConfig()
	result := ResolveSale(cfg, SaleClassificationInput{
		Country:     "DE",
		IsB2B:       true,
		GrossSEKOre: 12500,
		Evidence: SaleEvidence{
			CountryEvidence:        true,
			CustomerVATID:          "DE123456789",
			CustomerTaxExempt:      "reverse",
			AutomaticTaxEnabled:    true,
			AutomaticTaxStatus:     "complete",
			StripeTaxReverseCharge: true,
		},
	})

	if result.ReviewReason != "" {
		t.Fatalf("expected Stripe Tax reverse-charge sale to resolve, got %q", result.ReviewReason)
	}
	if result.MarketCode != "EU_B2B" {
		t.Fatalf("expected EU_B2B market, got %q", result.MarketCode)
	}
	if result.VATTreatment != "eu_reverse_charge" {
		t.Fatalf("expected eu_reverse_charge treatment, got %q", result.VATTreatment)
	}
}

func TestResolveSaleAllowsStripeTaxBackedEUB2C(t *testing.T) {
	cfg := testConfig()
	vat := int64(2100)
	result := ResolveSale(cfg, SaleClassificationInput{
		Country:           "DE",
		IsB2B:             false,
		GrossSEKOre:       12100,
		ExplicitVATSEKOre: &vat,
		Evidence: SaleEvidence{
			CountryEvidence:      true,
			AutomaticTaxEnabled:  true,
			AutomaticTaxStatus:   "complete",
			StripeTaxAmountKnown: true,
			AllowCountryFallback: true,
		},
	})

	if result.ReviewReason != "" {
		t.Fatalf("expected Stripe Tax EU B2C sale to resolve, got %q", result.ReviewReason)
	}
	if result.MarketCode != "DE" {
		t.Fatalf("expected market code DE, got %q", result.MarketCode)
	}
	if result.VATTreatment != "eu_b2c" {
		t.Fatalf("expected eu_b2c treatment, got %q", result.VATTreatment)
	}
	if result.VATSEKOre != vat {
		t.Fatalf("expected VAT %d ore, got %d", vat, result.VATSEKOre)
	}
}

func TestBuildReviewTransferFactsCreatesBalancedNeedsReviewPair(t *testing.T) {
	facts, err := BuildReviewTransferFacts(ReviewTransferInput{
		BokioCompanyID:    uuid.New(),
		StripeAccountID:   "self",
		SourceGroupID:     "charge:ch_123:sale",
		SourceObjectType:  "charge",
		SourceObjectID:    "ch_123",
		PostingDate:       time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		SourceCurrency:    "SEK",
		SourceAmountMinor: 12500,
		AmountSEKOre:      12500,
		DebitFactType:     "sale_review_receivable",
		CreditFactType:    "sale_review_obs",
		DebitAccount:      1580,
		CreditAccount:     2999,
		ReviewReason:      "missing VAT evidence",
		Payload:           map[string]any{"id": "ch_123"},
	})
	if err != nil {
		t.Fatalf("BuildReviewTransferFacts returned error: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("expected 2 review facts, got %d", len(facts))
	}
	if facts[0].Status != domain.FactStatusNeedsReview || facts[1].Status != domain.FactStatusNeedsReview {
		t.Fatalf("expected needs_review status on both facts")
	}
	if facts[0].Direction != domain.DirectionDebit || facts[1].Direction != domain.DirectionCredit {
		t.Fatalf("unexpected directions: %#v", facts)
	}
	if facts[0].AmountSEKOre != facts[1].AmountSEKOre {
		t.Fatalf("expected balanced review amounts, got %#v", facts)
	}
}

func testConfig() config.Config {
	return config.Config{
		Accounts: config.AccountsConfig{
			StripeReceivable:        1580,
			Bank:                    1920,
			Dispute:                 1510,
			FallbackOBS:             2999,
			Rounding:                3740,
			StripeBalanceByCurrency: map[string]int{"SEK": 1980},
			StripeFees:              config.StripeFeesConfig{Expense: 4535, InputVAT: 2645, OutputVAT: 2614},
			SalesByMarket: map[string]config.SalesMarketConfig{
				"SE":     {Revenue: 3001, OutputVAT: 2611, VATRatePercent: 25},
				"EU_B2B": {Revenue: 3308},
				"EXPORT": {Revenue: 3305},
			},
			OtherCountriesDefault: &config.SalesMarketConfig{Revenue: 3100, OutputVAT: 2614, VATRatePercent: 25},
		},
	}
}
