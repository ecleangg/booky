package tax

import (
	"encoding/json"
	"testing"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/google/uuid"
)

func TestBuildCaseCheckoutB2C(t *testing.T) {
	snapshots := []domain.ObjectSnapshot{
		snapshot(t, "checkout_session", "cs_123", map[string]any{
			"id":           "cs_123",
			"currency":     "eur",
			"amount_total": 11900,
			"customer_details": map[string]any{
				"address": map[string]any{"country": "DE"},
			},
			"automatic_tax": map[string]any{"enabled": true, "status": "complete"},
			"total_details": map[string]any{"amount_tax": 1900},
			"line_items": map[string]any{
				"data": []map[string]any{
					{
						"price": map[string]any{
							"product": map[string]any{
								"type": "service",
							},
						},
					},
				},
			},
		}),
	}

	result, err := BuildCase(uuid.New(), false, snapshots, nil, nil)
	if err != nil {
		t.Fatalf("BuildCase returned error: %v", err)
	}
	if result.Case.TaxStatus == nil || *result.Case.TaxStatus != "EU_DE_B2C" {
		t.Fatalf("expected EU_DE_B2C, got %#v", result.Case.TaxStatus)
	}
	if result.Case.ReportabilityState != domain.Reportable {
		t.Fatalf("expected reportable, got %q", result.Case.ReportabilityState)
	}
	if result.Case.SaleType == nil || *result.Case.SaleType != "SERVICES" {
		t.Fatalf("expected SERVICES sale type, got %#v", result.Case.SaleType)
	}
}

func TestBuildCaseCheckoutSEWithVATIsB2BAndUsesLineItemSaleType(t *testing.T) {
	snapshots := []domain.ObjectSnapshot{
		snapshot(t, "checkout_session", "cs_se", map[string]any{
			"id":           "cs_se",
			"currency":     "eur",
			"amount_total": 2249,
			"customer_details": map[string]any{
				"address": map[string]any{"country": "SE"},
				"tax_ids": []map[string]any{{"type": "eu_vat", "value": "SE123456789123"}},
			},
			"automatic_tax": map[string]any{"enabled": true, "status": "complete"},
			"total_details": map[string]any{"amount_tax": 450},
			"line_items": map[string]any{
				"data": []map[string]any{
					{
						"price": map[string]any{
							"product": map[string]any{
								"tax_code": "txcd_10103000",
								"type":     "service",
							},
						},
					},
				},
			},
		}),
	}

	result, err := BuildCase(uuid.New(), false, snapshots, nil, nil)
	if err != nil {
		t.Fatalf("BuildCase returned error: %v", err)
	}
	if result.Case.TaxStatus == nil || *result.Case.TaxStatus != domain.TaxStatusSEB2B {
		t.Fatalf("expected SE_B2B, got %#v", result.Case.TaxStatus)
	}
	if !result.Case.BuyerIsBusiness {
		t.Fatal("expected buyer to be recognized as business")
	}
	if result.Case.SaleType == nil || *result.Case.SaleType != "SERVICES" {
		t.Fatalf("expected SERVICES sale type, got %#v", result.Case.SaleType)
	}
	if result.Case.ReportabilityState != domain.Reportable {
		t.Fatalf("expected reportable, got %q", result.Case.ReportabilityState)
	}
}

func TestBuildCaseInvoiceB2BWithVerifiedVAT(t *testing.T) {
	snapshots := []domain.ObjectSnapshot{
		snapshot(t, "invoice", "in_123", map[string]any{
			"id":                  "in_123",
			"customer":            "cus_123",
			"currency":            "eur",
			"total":               10000,
			"customer_address":    map[string]any{"country": "DE"},
			"customer_tax_exempt": "reverse",
			"customer_tax_ids":    []map[string]any{{"type": "eu_vat", "value": "DE123456789"}},
			"automatic_tax":       map[string]any{"enabled": true, "status": "complete"},
			"lines": map[string]any{
				"data": []map[string]any{
					{"taxes": []map[string]any{{"amount": 0, "taxability_reason": "reverse_charge"}}},
				},
			},
			"metadata": map[string]any{"sale_category": "services"},
		}),
		snapshot(t, "tax_id", "txi_123", map[string]any{
			"id":           "txi_123",
			"customer":     "cus_123",
			"type":         "eu_vat",
			"value":        "DE123456789",
			"verification": map[string]any{"status": "verified"},
		}),
	}

	result, err := BuildCase(uuid.New(), false, snapshots, nil, nil)
	if err != nil {
		t.Fatalf("BuildCase returned error: %v", err)
	}
	if result.Case.TaxStatus == nil || *result.Case.TaxStatus != "EU_DE_B2B" {
		t.Fatalf("expected EU_DE_B2B, got %#v", result.Case.TaxStatus)
	}
	if !result.Case.BuyerVATVerified {
		t.Fatalf("expected VAT verification to be preserved")
	}
	if result.Case.ReportabilityState != domain.Reportable {
		t.Fatalf("expected reportable, got %q", result.Case.ReportabilityState)
	}
}

func TestBuildCaseRequiresManualEvidenceForUnverifiedEUB2B(t *testing.T) {
	snapshots := []domain.ObjectSnapshot{
		snapshot(t, "invoice", "in_123", map[string]any{
			"id":                  "in_123",
			"customer_address":    map[string]any{"country": "DE"},
			"customer_tax_exempt": "reverse",
			"customer_tax_ids":    []map[string]any{{"type": "eu_vat", "value": "DE123456789"}},
			"automatic_tax":       map[string]any{"enabled": true, "status": "complete"},
			"lines": map[string]any{
				"data": []map[string]any{
					{"taxes": []map[string]any{{"amount": 0, "taxability_reason": "reverse_charge"}}},
				},
			},
			"metadata": map[string]any{"sale_category": "services"},
		}),
	}

	result, err := BuildCase(uuid.New(), false, snapshots, nil, nil)
	if err != nil {
		t.Fatalf("BuildCase returned error: %v", err)
	}
	if result.Case.ReportabilityState != domain.NeedsManualEvidence {
		t.Fatalf("expected needs_manual_evidence, got %q", result.Case.ReportabilityState)
	}
}

func TestBuildCaseUsesManualCountryFallback(t *testing.T) {
	manual := []domain.ManualTaxEvidence{{
		TaxCaseID:       uuid.New(),
		Country:         strPtr("NO"),
		CountrySource:   strPtr("manual"),
		BuyerIsBusiness: boolPtr(false),
		SaleType:        strPtr("services"),
	}}
	snapshots := []domain.ObjectSnapshot{
		snapshot(t, "payment_intent", "pi_123", map[string]any{
			"id":       "pi_123",
			"currency": "nok",
			"amount":   2500,
		}),
	}

	result, err := BuildCase(uuid.New(), false, snapshots, manual, nil)
	if err != nil {
		t.Fatalf("BuildCase returned error: %v", err)
	}
	if result.Case.TaxStatus == nil || *result.Case.TaxStatus != domain.TaxStatusOutsideEU {
		t.Fatalf("expected OUTSIDE_EU, got %#v", result.Case.TaxStatus)
	}
	if result.Case.Country == nil || *result.Case.Country != "NO" {
		t.Fatalf("expected manual country NO, got %#v", result.Case.Country)
	}
}

func snapshot(t *testing.T, objectType, objectID string, payload map[string]any) domain.ObjectSnapshot {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	return domain.ObjectSnapshot{ObjectType: objectType, ObjectID: objectID, Payload: b}
}

func boolPtr(value bool) *bool {
	return &value
}
