package stripe

import (
	"testing"

	"github.com/ecleangg/booky/internal/domain"
)

func TestBuildSaleClassificationInputUsesStripeTaxInvoiceEvidence(t *testing.T) {
	rate := 11.0
	charge := Charge{
		ID:       "ch_123",
		Amount:   11900,
		Currency: "eur",
	}
	bundle := chargeEvidenceBundle{
		Invoice: &Invoice{
			CustomerAddress: &address{Country: "DE"},
			AutomaticTax:    invoiceAutomaticTax{Enabled: true, Status: "complete"},
			Lines: invoiceLineCollection{
				Data: []InvoiceLine{
					{Taxes: []InvoiceLineTax{{Amount: 1900, TaxabilityReason: "standard_rated"}}},
				},
			},
		},
	}
	bt := domain.BalanceTransaction{
		Currency:     "EUR",
		AmountMinor:  11900,
		ExchangeRate: &rate,
	}

	input := buildSaleClassificationInput(charge, bundle, bt)

	if input.Country != "DE" {
		t.Fatalf("expected DE country, got %q", input.Country)
	}
	if input.IsB2B {
		t.Fatalf("expected B2C classification")
	}
	if input.ExplicitVATSEKOre == nil || *input.ExplicitVATSEKOre != 20900 {
		t.Fatalf("expected VAT 20900 ore, got %#v", input.ExplicitVATSEKOre)
	}
	if !input.Evidence.AutomaticTaxEnabled || input.Evidence.AutomaticTaxStatus != "complete" {
		t.Fatalf("expected automatic tax evidence, got %#v", input.Evidence)
	}
	if !input.Evidence.StripeTaxAmountKnown {
		t.Fatalf("expected Stripe tax amount to be known")
	}
	if !input.Evidence.AllowCountryFallback {
		t.Fatalf("expected automatic tax to allow country fallback")
	}
}

func TestSaleEvidenceFromChargeUsesInvoiceVATIDAndVerification(t *testing.T) {
	charge := Charge{CustomerTaxExempt: "reverse"}
	bundle := chargeEvidenceBundle{
		Invoice: &Invoice{
			CustomerAddress:   &address{Country: "DE"},
			CustomerTaxExempt: "reverse",
			CustomerTaxIDs:    []InvoiceCustomerTaxID{{Type: "eu_vat", Value: "DE123456789"}},
			AutomaticTax:      invoiceAutomaticTax{Enabled: true, Status: "complete"},
			Lines: invoiceLineCollection{
				Data: []InvoiceLine{
					{Taxes: []InvoiceLineTax{{Amount: 0, TaxabilityReason: "reverse_charge"}}},
				},
			},
		},
		CustomerTaxIDs: []TaxID{
			{Type: "eu_vat", Value: "DE123456789", Verification: &taxIDVerification{Status: "verified"}},
		},
	}

	evidence := saleEvidenceFromCharge(charge, bundle)
	_, isB2B := saleClassificationInputs(charge, bundle)

	if !isB2B {
		t.Fatalf("expected reverse-charge evidence to classify sale as B2B")
	}
	if evidence.CustomerVATID != "DE123456789" {
		t.Fatalf("expected VAT ID from invoice/customer evidence, got %q", evidence.CustomerVATID)
	}
	if !evidence.CustomerVATValidated {
		t.Fatalf("expected VAT ID verification to be preserved")
	}
	if !evidence.StripeTaxReverseCharge {
		t.Fatalf("expected reverse-charge taxability evidence")
	}
}

func TestSaleClassificationInputsPreferShippingCountryForGoods(t *testing.T) {
	charge := Charge{
		BillingDetails: billingDetails{Address: address{Country: "FR"}},
		Metadata:       map[string]string{"sale_category": "goods"},
	}
	bundle := chargeEvidenceBundle{
		Invoice: &Invoice{
			CustomerAddress:  &address{Country: "FR"},
			CustomerShipping: &shipping{Address: address{Country: "DE"}},
		},
	}

	country, _ := saleClassificationInputs(charge, bundle)

	if country != "DE" {
		t.Fatalf("expected shipping country DE for goods sale, got %q", country)
	}
}
