package filings

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/support"
)

func isOSSCandidate(input filingContext) bool {
	vatTreatment := normalizeVATMode(input.VATTreatment)
	if vatTreatment == "import_oss" || vatTreatment == "ioss" {
		return true
	}
	if vatTreatment == "eu_oss" || support.MapTruthy(input.AllMetadata, "booky_oss_applied", "oss_applied") {
		return support.IsEUCountry(input.Country) && input.Country != "SE"
	}
	return false
}

func isPSCandidate(input filingContext) bool {
	if strings.EqualFold(strings.TrimSpace(input.MarketCode), "EU_B2B") {
		return true
	}
	if input.BuyerVATNumber == "" {
		return false
	}
	cc := support.CountryPrefix(input.BuyerVATNumber)
	return support.IsEUCountry(cc) && cc != "SE"
}

func resolveSaleType(saleCategory string, shippingEvidence bool) (string, string) {
	switch strings.ToLower(strings.TrimSpace(saleCategory)) {
	case "goods", "physical_goods", "physical":
		return "GOODS", ""
	case "":
		if shippingEvidence {
			return "", "shipping evidence suggests goods but sale_category is missing"
		}
		return "SERVICES", ""
	default:
		return "SERVICES", ""
	}
}

func mapPSSaleType(saleType string) string {
	switch saleType {
	case "GOODS":
		return "goods"
	case "SERVICES":
		return "services"
	default:
		return ""
	}
}

func filingUnsupportedReason(input filingContext) string {
	metadata := input.AllMetadata
	if containsMetadataValue(metadata, []string{"booky_vat_mode", "vat_mode"}, []string{"import_oss", "ioss", "import"}) {
		return "import OSS is unsupported in v1"
	}
	if containsMetadataValue(metadata, []string{"triangulation", "booky_triangulation", "triangulation_code"}, []string{"1", "true", "x"}) {
		return "triangulation is unsupported in v1"
	}
	if containsMetadataValue(metadata, []string{"booky_call_off_stock", "call_off_stock", "avropslager"}, []string{"1", "true", "x", "y", "z"}) {
		return "call-off stock / avropslager is unsupported in v1"
	}
	if containsMetadataValue(metadata, []string{"periodic_summary_marker", "ps_marker"}, []string{"x", "y", "z"}) {
		return "call-off stock / avropslager markers X/Y/Z are unsupported in v1"
	}
	if origin := support.NormalizeCountry(support.MapString(metadata, "booky_origin_country", "origin_country", "dispatch_from_country", "establishment_country")); origin != "" && origin != "SE" {
		return "non-Swedish OSS origin establishment is unsupported in v1"
	}
	if isPSCandidate(input) && strings.TrimSpace(input.BuyerVATNumber) == "" {
		return "EU B2B sale is missing buyer VAT ID for periodic summary"
	}
	return ""
}

func snapshotKey(objectType, objectID string) string {
	return objectType + ":" + objectID
}

func decodeCharge(raw json.RawMessage) (chargePayload, error) {
	var charge chargePayload
	if err := json.Unmarshal(raw, &charge); err != nil {
		return chargePayload{}, fmt.Errorf("decode charge payload: %w", err)
	}
	return charge, nil
}

func decodeRefund(raw json.RawMessage) (refundPayload, error) {
	var refund refundPayload
	if err := json.Unmarshal(raw, &refund); err != nil {
		return refundPayload{}, fmt.Errorf("decode refund payload: %w", err)
	}
	return refund, nil
}

func decodeInvoice(raw json.RawMessage) (invoicePayload, error) {
	var invoice invoicePayload
	if err := json.Unmarshal(raw, &invoice); err != nil {
		return invoicePayload{}, fmt.Errorf("decode invoice payload: %w", err)
	}
	return invoice, nil
}

func decodeCustomer(raw json.RawMessage) (customerPayload, error) {
	var customer customerPayload
	if err := json.Unmarshal(raw, &customer); err != nil {
		return customerPayload{}, fmt.Errorf("decode customer payload: %w", err)
	}
	return customer, nil
}

func metadataFromInvoice(invoice *invoicePayload) map[string]string {
	if invoice == nil {
		return nil
	}
	return invoice.Metadata
}

func metadataFromCustomer(customer *customerPayload) map[string]string {
	if customer == nil {
		return nil
	}
	return customer.Metadata
}

func resolveCountryEvidence(charge chargePayload, invoice *invoicePayload, customer *customerPayload, saleCategory string, metadata map[string]string) string {
	if country := support.NormalizeCountry(support.MapString(metadata, "market_country")); country != "" {
		return country
	}
	if country := support.NormalizeCountry(support.MapString(metadata, "market_code")); len(country) == 2 {
		return country
	}
	preferShipping := support.IsGoodsCategory(saleCategory)
	if preferShipping {
		if country := invoiceShippingCountry(invoice); country != "" {
			return country
		}
		if country := customerShippingCountry(customer); country != "" {
			return country
		}
	}
	if country := invoiceCustomerCountry(invoice); country != "" {
		return country
	}
	if country := customerCountry(customer); country != "" {
		return country
	}
	if !preferShipping {
		if country := invoiceShippingCountry(invoice); country != "" {
			return country
		}
		if country := customerShippingCountry(customer); country != "" {
			return country
		}
	}
	if charge.CustomerDetails != nil && strings.TrimSpace(charge.CustomerDetails.Address.Country) != "" {
		return support.NormalizeCountry(charge.CustomerDetails.Address.Country)
	}
	if strings.TrimSpace(charge.BillingDetails.Address.Country) != "" {
		return support.NormalizeCountry(charge.BillingDetails.Address.Country)
	}
	return ""
}

func resolveCustomerVATID(metadata map[string]string, invoice *invoicePayload) string {
	if value := strings.TrimSpace(support.MapString(metadata, "booky_customer_vat_id", "customer_vat_id")); value != "" {
		return strings.ToUpper(value)
	}
	if invoice != nil {
		for _, taxID := range invoice.CustomerTaxIDs {
			if strings.EqualFold(strings.TrimSpace(taxID.Type), "eu_vat") && strings.TrimSpace(taxID.Value) != "" {
				return strings.ToUpper(strings.TrimSpace(taxID.Value))
			}
		}
	}
	return ""
}

func hasShippingEvidence(invoice *invoicePayload, customer *customerPayload) bool {
	return invoiceShippingCountry(invoice) != "" || customerShippingCountry(customer) != ""
}

func invoiceCustomerCountry(invoice *invoicePayload) string {
	if invoice == nil || invoice.CustomerAddress == nil {
		return ""
	}
	return support.NormalizeCountry(invoice.CustomerAddress.Country)
}

func invoiceShippingCountry(invoice *invoicePayload) string {
	if invoice == nil || invoice.CustomerShipping == nil {
		return ""
	}
	return support.NormalizeCountry(invoice.CustomerShipping.Address.Country)
}

func customerCountry(customer *customerPayload) string {
	if customer == nil || customer.Address == nil {
		return ""
	}
	return support.NormalizeCountry(customer.Address.Country)
}

func customerShippingCountry(customer *customerPayload) string {
	if customer == nil || customer.Shipping == nil {
		return ""
	}
	return support.NormalizeCountry(customer.Shipping.Address.Country)
}

func normalizeVATMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "oss", "eu_oss":
		return "eu_oss"
	case "eu_b2c":
		return "eu_b2c"
	case "eu_b2b", "eu_reverse_charge", "reverse_charge":
		return "eu_reverse_charge"
	case "ioss", "import_oss":
		return "import_oss"
	default:
		return strings.ToLower(strings.TrimSpace(mode))
	}
}

func vatRateBasisPoints(revenueSEKOre, vatSEKOre int64) int {
	if revenueSEKOre == 0 || vatSEKOre == 0 {
		return 0
	}
	return int(math.Round((float64(vatSEKOre) / float64(revenueSEKOre)) * 10000))
}

func containsMetadataValue(metadata map[string]string, keys, values []string) bool {
	allowed := map[string]struct{}{}
	for _, value := range values {
		allowed[strings.ToLower(strings.TrimSpace(value))] = struct{}{}
	}
	for _, key := range keys {
		value := strings.ToLower(strings.TrimSpace(metadata[key]))
		if value == "" {
			continue
		}
		if _, ok := allowed[value]; ok {
			return true
		}
	}
	return false
}

func filingSourceGroups(facts []domain.AccountingFact) []string {
	set := map[string]struct{}{}
	for _, fact := range facts {
		if isFilingSourceGroup(fact.SourceGroupID) {
			set[fact.SourceGroupID] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for groupID := range set {
		out = append(out, groupID)
	}
	sort.Strings(out)
	return out
}

func isFilingSourceGroup(sourceGroupID string) bool {
	return strings.HasPrefix(sourceGroupID, "refund:") || strings.HasSuffix(sourceGroupID, ":sale")
}

func filterOSSPeriodEntries(entries []domain.OSSUnionEntry, kind, period string) []domain.OSSUnionEntry {
	if kind != domain.FilingKindOSSUnion {
		return nil
	}
	filtered := make([]domain.OSSUnionEntry, 0)
	for _, entry := range entries {
		if entry.FilingPeriod == period {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func filterPSPeriodEntries(entries []domain.PeriodicSummaryEntry, kind, period string) []domain.PeriodicSummaryEntry {
	if kind != domain.FilingKindPeriodicSummary {
		return nil
	}
	filtered := make([]domain.PeriodicSummaryEntry, 0)
	for _, entry := range entries {
		if entry.FilingPeriod == period {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func filterPeriods(periods []domain.FilingPeriod, kind, period string) []domain.FilingPeriod {
	filtered := make([]domain.FilingPeriod, 0)
	for _, filingPeriod := range periods {
		if filingPeriod.Kind == kind && filingPeriod.Period == period {
			filtered = append(filtered, filingPeriod)
		}
	}
	return filtered
}

func periodKey(kind, period string) string {
	return kind + ":" + period
}
