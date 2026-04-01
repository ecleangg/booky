package accounting

import (
	"fmt"
	"math"
	"strings"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/support"
)

func ResolveSale(cfg config.Config, input SaleClassificationInput) SalesResolution {
	country := strings.ToUpper(strings.TrimSpace(input.Country))
	if country == "" || !input.Evidence.CountryEvidence {
		return SalesResolution{ReviewReason: "missing customer country evidence"}
	}

	marketCode, treatment := ClassifyMarket(country, input.IsB2B)
	if reason := validateAutoPostingEvidence(marketCode, input); reason != "" {
		return SalesResolution{ReviewReason: reason}
	}

	marketCfg, ok := cfg.Accounts.SalesByMarket[marketCode]
	if !ok && cfg.Accounts.OtherCountriesDefault != nil && shouldUseOtherCountriesDefault(marketCode) && input.Evidence.AllowCountryFallback {
		marketCfg = *cfg.Accounts.OtherCountriesDefault
		ok = true
	}
	if !ok {
		return SalesResolution{ReviewReason: fmt.Sprintf("no sales account mapping for market %s", marketCode)}
	}

	res := SalesResolution{
		MarketCode:       marketCode,
		VATTreatment:     resolveVATTreatment(marketCode, treatment, input),
		RevenueAccount:   marketCfg.Revenue,
		OutputVATAccount: marketCfg.OutputVAT,
	}
	if marketCfg.OutputVAT == 0 {
		res.RevenueSEKOre = input.GrossSEKOre
		return res
	}
	if input.ExplicitVATSEKOre != nil {
		res.VATSEKOre = *input.ExplicitVATSEKOre
		res.RevenueSEKOre = input.GrossSEKOre - *input.ExplicitVATSEKOre
		return res
	}
	if marketCfg.VATRatePercent <= 0 {
		res.ReviewReason = fmt.Sprintf("market %s requires VAT amount or vat_rate_percent", marketCode)
		return res
	}

	vat := int64(math.Round(float64(input.GrossSEKOre) * marketCfg.VATRatePercent / (100.0 + marketCfg.VATRatePercent)))
	res.VATSEKOre = vat
	res.RevenueSEKOre = input.GrossSEKOre - vat
	return res
}

func shouldUseOtherCountriesDefault(marketCode string) bool {
	switch marketCode {
	case "SE", "EU_B2B", "EXPORT":
		return false
	default:
		return len(marketCode) == 2
	}
}

func ClassifyMarket(country string, isB2B bool) (string, string) {
	cc := strings.ToUpper(strings.TrimSpace(country))
	if cc == "" {
		return "UNKNOWN", "review"
	}
	if cc == "SE" {
		return "SE", "domestic"
	}
	if isB2B && support.IsEUCountry(cc) {
		return "EU_B2B", "eu_b2b"
	}
	if support.IsEUCountry(cc) {
		return cc, "eu_b2c"
	}
	return "EXPORT", "export"
}

func validateAutoPostingEvidence(marketCode string, input SaleClassificationInput) string {
	mode := normalizeVATMode(input.Evidence.VATMode)

	if input.Evidence.AutomaticTaxEnabled {
		switch strings.ToLower(strings.TrimSpace(input.Evidence.AutomaticTaxStatus)) {
		case "failed":
			return "Stripe Tax calculation failed"
		case "requires_location_inputs":
			return "Stripe Tax requires complete customer location inputs before auto-posting"
		}
	}

	switch marketCode {
	case "SE":
		if mode != "" && mode != "domestic" {
			return fmt.Sprintf("unsupported VAT mode %q for Swedish domestic sale", input.Evidence.VATMode)
		}
	case "EU_B2B":
		if mode != "" && mode != "eu_b2b" && mode != "eu_reverse_charge" {
			return fmt.Sprintf("unsupported VAT mode %q for EU B2B sale", input.Evidence.VATMode)
		}
		if !hasEUB2BReverseChargeEvidence(input.Evidence) {
			return "EU B2B sale requires Stripe Tax reverse-charge evidence or a validated customer VAT ID"
		}
	case "EXPORT":
		if mode == "export" || mode == "export_goods" || mode == "export_services" {
			return ""
		}
		if !input.Evidence.ExportEvidence {
			return "export sale requires explicit export evidence before auto-posting"
		}
	default:
		if len(marketCode) == 2 {
			if mode != "" && mode != "eu_b2c" && mode != "eu_oss" && mode != "eu_local_vat" {
				return fmt.Sprintf("unsupported VAT mode %q for EU B2C sale", input.Evidence.VATMode)
			}
			if !input.Evidence.OSSApplied && mode != "eu_oss" && mode != "eu_local_vat" && !hasEUB2CTaxEvidence(input) {
				return fmt.Sprintf("EU B2C sale to %s requires Stripe Tax, OSS, or local VAT evidence", marketCode)
			}
		}
	}

	return ""
}

func resolveVATTreatment(marketCode, baseTreatment string, input SaleClassificationInput) string {
	switch normalizeVATMode(input.Evidence.VATMode) {
	case "domestic":
		return "domestic"
	case "eu_reverse_charge":
		return "eu_reverse_charge"
	case "eu_oss":
		return "eu_oss"
	case "eu_local_vat":
		return "eu_local_vat"
	case "export", "export_goods", "export_services":
		return normalizeVATMode(input.Evidence.VATMode)
	}

	switch marketCode {
	case "EU_B2B":
		if hasEUB2BReverseChargeEvidence(input.Evidence) {
			return "eu_reverse_charge"
		}
	case "SE":
		return "domestic"
	default:
		if len(marketCode) == 2 && support.IsEUCountry(marketCode) {
			if input.Evidence.OSSApplied {
				return "eu_oss"
			}
			if normalizeVATMode(input.Evidence.VATMode) == "eu_local_vat" {
				return "eu_local_vat"
			}
		}
	}
	return baseTreatment
}

func normalizeVATMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "":
		return ""
	case "domestic", "domestic_se":
		return "domestic"
	case "eu_b2b", "reverse_charge", "eu_reverse_charge":
		return "eu_reverse_charge"
	case "oss", "eu_oss":
		return "eu_oss"
	case "local_vat", "eu_local_vat":
		return "eu_local_vat"
	case "eu_b2c":
		return "eu_b2c"
	default:
		return strings.ToLower(strings.TrimSpace(mode))
	}
}

func hasEUB2BReverseChargeEvidence(evidence SaleEvidence) bool {
	if strings.TrimSpace(evidence.CustomerVATID) == "" {
		return false
	}
	if evidence.CustomerVATValidated {
		return true
	}
	if automaticTaxComplete(evidence) && (evidence.StripeTaxReverseCharge || strings.EqualFold(strings.TrimSpace(evidence.CustomerTaxExempt), "reverse")) {
		return true
	}
	return false
}

func hasEUB2CTaxEvidence(input SaleClassificationInput) bool {
	if !automaticTaxComplete(input.Evidence) {
		return false
	}
	if input.ExplicitVATSEKOre != nil {
		return true
	}
	return input.Evidence.StripeTaxAmountKnown || input.Evidence.StripeTaxZeroRated
}

func automaticTaxComplete(evidence SaleEvidence) bool {
	return evidence.AutomaticTaxEnabled && strings.EqualFold(strings.TrimSpace(evidence.AutomaticTaxStatus), "complete")
}
