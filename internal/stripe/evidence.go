package stripe

import (
	"strconv"
	"strings"

	"github.com/ecleangg/booky/internal/accounting"
	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/support"
)

func buildSaleClassificationInput(charge Charge, bundle chargeEvidenceBundle, bt domain.BalanceTransaction) accounting.SaleClassificationInput {
	country, isB2B := saleClassificationInputs(charge, bundle)
	return accounting.SaleClassificationInput{
		Country:           country,
		IsB2B:             isB2B,
		GrossSEKOre:       settledGrossSEKOre(bt),
		ExplicitVATSEKOre: explicitVATFromChargeEvidence(charge, bundle, bt),
		Evidence:          saleEvidenceFromCharge(charge, bundle),
	}
}

func explicitVATFromChargeEvidence(charge Charge, bundle chargeEvidenceBundle, bt domain.BalanceTransaction) *int64 {
	if vat := explicitVATFromStripeInvoice(bundle.Invoice, bt); vat != nil {
		return vat
	}
	return explicitVATFromMetadata(support.MergeStringMaps(metadataFromCustomer(bundle.Customer), metadataFromInvoice(bundle.Invoice), charge.Metadata), bt)
}

func explicitVATFromStripeInvoice(invoice *Invoice, bt domain.BalanceTransaction) *int64 {
	known, totalMinor, _ := invoiceTaxSummary(invoice)
	if !known {
		return nil
	}
	totalSEK := totalMinor
	if strings.ToUpper(bt.Currency) != "SEK" {
		totalSEK = amountToSEKOre(totalMinor, bt)
	}
	return &totalSEK
}

func explicitVATFromMetadata(metadata map[string]string, bt domain.BalanceTransaction) *int64 {
	for _, key := range []string{"tax_amount_ore", "vat_amount_ore"} {
		value, ok := metadata[key]
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			result := parsed
			return &result
		}
	}
	for _, key := range []string{"tax_amount_minor", "vat_amount_minor"} {
		value, ok := metadata[key]
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			result := amountToSEKOre(parsed, bt)
			return &result
		}
	}
	return nil
}

func saleClassificationInputs(charge Charge, bundle chargeEvidenceBundle) (string, bool) {
	metadata := support.MergeStringMaps(metadataFromCustomer(bundle.Customer), metadataFromInvoice(bundle.Invoice), charge.Metadata)
	saleCategory := support.MapString(metadata, "booky_sale_category", "sale_category")
	country, _ := resolveCountryEvidence(charge, bundle, saleCategory, metadata)
	return country, isB2BSale(charge, bundle, metadata)
}

func saleEvidenceFromCharge(charge Charge, bundle chargeEvidenceBundle) accounting.SaleEvidence {
	metadata := support.MergeStringMaps(metadataFromCustomer(bundle.Customer), metadataFromInvoice(bundle.Invoice), charge.Metadata)
	saleCategory := support.MapString(metadata, "booky_sale_category", "sale_category")
	country, countrySource := resolveCountryEvidence(charge, bundle, saleCategory, metadata)
	customerVATID, customerVATValidated := resolveCustomerVATEvidence(metadata, bundle)
	stripeTaxKnown, _, reasons := invoiceTaxSummary(bundle.Invoice)
	customerTaxExempt := customerTaxExemptFromEvidence(charge, bundle)
	automaticTaxEnabled := bundle.Invoice != nil && bundle.Invoice.AutomaticTax.Enabled
	automaticTaxStatus := ""
	if bundle.Invoice != nil {
		automaticTaxStatus = strings.TrimSpace(bundle.Invoice.AutomaticTax.Status)
	}

	return accounting.SaleEvidence{
		CountryEvidence:        chargeHasCountryEvidence(charge, bundle),
		CountrySource:          countrySource,
		VATMode:                support.MapString(metadata, "booky_vat_mode", "vat_mode"),
		SaleCategory:           saleCategory,
		OSSApplied:             support.MapTruthy(metadata, "booky_oss_applied", "oss_applied"),
		ExportEvidence:         support.MapTruthy(metadata, "booky_export_evidence", "export_evidence"),
		CustomerVATID:          customerVATID,
		CustomerVATValidated:   customerVATValidated,
		CustomerTaxExempt:      customerTaxExempt,
		AutomaticTaxEnabled:    automaticTaxEnabled,
		AutomaticTaxStatus:     automaticTaxStatus,
		StripeTaxAmountKnown:   stripeTaxKnown,
		StripeTaxReverseCharge: hasInvoiceTaxabilityReason(bundle.Invoice, "reverse_charge") || (stripeTaxKnown && strings.EqualFold(customerTaxExempt, "reverse") && customerVATID != ""),
		StripeTaxZeroRated:     hasInvoiceTaxabilityReason(bundle.Invoice, "zero_rated"),
		TaxabilityReasons:      reasons,
		AllowCountryFallback:   support.MapTruthy(metadata, "booky_allow_country_fallback", "allow_country_fallback") || (automaticTaxEnabled && strings.EqualFold(automaticTaxStatus, "complete") && country != ""),
	}
}

func chargeHasCountryEvidence(charge Charge, bundle chargeEvidenceBundle) bool {
	metadata := support.MergeStringMaps(metadataFromCustomer(bundle.Customer), metadataFromInvoice(bundle.Invoice), charge.Metadata)
	country, _ := resolveCountryEvidence(charge, bundle, support.MapString(metadata, "booky_sale_category", "sale_category"), metadata)
	return country != ""
}

func resolveCountryEvidence(charge Charge, bundle chargeEvidenceBundle, saleCategory string, metadata map[string]string) (string, string) {
	if country := support.NormalizeCountry(support.MapString(metadata, "market_country")); country != "" {
		return country, "metadata.market_country"
	}
	if country := support.NormalizeCountry(support.MapString(metadata, "market_code")); len(country) == 2 {
		return country, "metadata.market_code"
	}

	preferShipping := isGoodsSale(saleCategory)
	if preferShipping {
		if country := invoiceShippingCountry(bundle.Invoice); country != "" {
			return country, "invoice.customer_shipping"
		}
		if country := customerShippingCountry(bundle.Customer); country != "" {
			return country, "customer.shipping"
		}
	}
	if country := invoiceCustomerCountry(bundle.Invoice); country != "" {
		return country, "invoice.customer_address"
	}
	if country := customerCountry(bundle.Customer); country != "" {
		return country, "customer.address"
	}
	if !preferShipping {
		if country := invoiceShippingCountry(bundle.Invoice); country != "" {
			return country, "invoice.customer_shipping"
		}
		if country := customerShippingCountry(bundle.Customer); country != "" {
			return country, "customer.shipping"
		}
	}
	if charge.CustomerDetails != nil && strings.TrimSpace(charge.CustomerDetails.Address.Country) != "" {
		return support.NormalizeCountry(charge.CustomerDetails.Address.Country), "charge.customer_details"
	}
	if strings.TrimSpace(charge.BillingDetails.Address.Country) != "" {
		return support.NormalizeCountry(charge.BillingDetails.Address.Country), "charge.billing_details"
	}
	return "", ""
}

func isB2BSale(charge Charge, bundle chargeEvidenceBundle, metadata map[string]string) bool {
	if raw := strings.TrimSpace(metadata["is_b2b"]); raw != "" {
		return support.ParseBool(raw)
	}
	customerVATID, customerVATValidated := resolveCustomerVATEvidence(metadata, bundle)
	if customerVATValidated {
		return true
	}
	return customerVATID != "" && strings.EqualFold(customerTaxExemptFromEvidence(charge, bundle), "reverse")
}

func customerTaxExemptFromEvidence(charge Charge, bundle chargeEvidenceBundle) string {
	if bundle.Invoice != nil && strings.TrimSpace(bundle.Invoice.CustomerTaxExempt) != "" {
		return strings.TrimSpace(bundle.Invoice.CustomerTaxExempt)
	}
	if strings.TrimSpace(charge.CustomerTaxExempt) != "" {
		return strings.TrimSpace(charge.CustomerTaxExempt)
	}
	if bundle.Customer != nil {
		return strings.TrimSpace(bundle.Customer.TaxExempt)
	}
	return ""
}

func resolveCustomerVATEvidence(metadata map[string]string, bundle chargeEvidenceBundle) (string, bool) {
	if value := strings.TrimSpace(support.MapString(metadata, "booky_customer_vat_id", "customer_vat_id")); value != "" {
		return strings.ToUpper(value), support.MapTruthy(metadata, "booky_customer_vat_valid", "customer_vat_valid")
	}
	if bundle.Invoice != nil {
		for _, taxID := range bundle.Invoice.CustomerTaxIDs {
			if !isEUVATTaxID(taxID.Type) || strings.TrimSpace(taxID.Value) == "" {
				continue
			}
			return strings.ToUpper(strings.TrimSpace(taxID.Value)), matchingTaxIDVerified(bundle.CustomerTaxIDs, taxID.Value)
		}
	}
	for _, taxID := range bundle.CustomerTaxIDs {
		if !isEUVATTaxID(taxID.Type) || strings.TrimSpace(taxID.Value) == "" {
			continue
		}
		return strings.ToUpper(strings.TrimSpace(taxID.Value)), taxIDVerified(taxID)
	}
	return "", false
}

func invoiceTaxSummary(invoice *Invoice) (bool, int64, []string) {
	if invoice == nil {
		return false, 0, nil
	}
	var total int64
	reasonSet := map[string]struct{}{}
	known := false
	for _, line := range invoice.Lines.Data {
		for _, tax := range line.Taxes {
			known = true
			total += tax.Amount
			if reason := strings.TrimSpace(tax.TaxabilityReason); reason != "" {
				reasonSet[strings.ToLower(reason)] = struct{}{}
			}
		}
	}
	if !known && invoice.AutomaticTax.Enabled && strings.EqualFold(strings.TrimSpace(invoice.AutomaticTax.Status), "complete") {
		known = true
	}
	reasons := make([]string, 0, len(reasonSet))
	for reason := range reasonSet {
		reasons = append(reasons, reason)
	}
	return known, total, reasons
}

func hasInvoiceTaxabilityReason(invoice *Invoice, wanted string) bool {
	_, _, reasons := invoiceTaxSummary(invoice)
	for _, reason := range reasons {
		if reason == strings.ToLower(strings.TrimSpace(wanted)) {
			return true
		}
	}
	return false
}

func metadataFromInvoice(invoice *Invoice) map[string]string {
	if invoice == nil {
		return nil
	}
	return invoice.Metadata
}

func metadataFromCustomer(customer *Customer) map[string]string {
	if customer == nil {
		return nil
	}
	return customer.Metadata
}

func invoiceCustomerCountry(invoice *Invoice) string {
	if invoice == nil || invoice.CustomerAddress == nil {
		return ""
	}
	return support.NormalizeCountry(invoice.CustomerAddress.Country)
}

func invoiceShippingCountry(invoice *Invoice) string {
	if invoice == nil || invoice.CustomerShipping == nil {
		return ""
	}
	return support.NormalizeCountry(invoice.CustomerShipping.Address.Country)
}

func customerCountry(customer *Customer) string {
	if customer == nil || customer.Address == nil {
		return ""
	}
	return support.NormalizeCountry(customer.Address.Country)
}

func customerShippingCountry(customer *Customer) string {
	if customer == nil || customer.Shipping == nil {
		return ""
	}
	return support.NormalizeCountry(customer.Shipping.Address.Country)
}

func isGoodsSale(category string) bool {
	return support.IsGoodsCategory(category)
}

func isEUVATTaxID(kind string) bool {
	return strings.EqualFold(strings.TrimSpace(kind), "eu_vat")
}

func matchingTaxIDVerified(taxIDs []TaxID, value string) bool {
	target := strings.ToUpper(strings.TrimSpace(value))
	for _, taxID := range taxIDs {
		if strings.ToUpper(strings.TrimSpace(taxID.Value)) == target {
			return taxIDVerified(taxID)
		}
	}
	return false
}

func taxIDVerified(taxID TaxID) bool {
	return taxID.Verification != nil && strings.EqualFold(strings.TrimSpace(taxID.Verification.Status), "verified")
}
