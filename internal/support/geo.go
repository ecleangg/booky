package support

import "strings"

func NormalizeCountry(country string) string {
	return strings.ToUpper(strings.TrimSpace(country))
}

func CountryPrefix(value string) string {
	if len(value) < 2 {
		return ""
	}
	return NormalizeCountry(value[:2])
}

func IsGoodsCategory(category string) bool {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "goods", "physical_goods", "physical":
		return true
	default:
		return false
	}
}

func IsEUCountry(country string) bool {
	switch NormalizeCountry(country) {
	case "AT", "AX", "BE", "BG", "HR", "CY", "CZ", "DE", "DK", "EE", "EL", "ES", "FI", "FR", "GR", "HU", "IE", "IT", "LT", "LU", "LV", "MT", "NL", "PL", "PT", "RO", "SE", "SI", "SK":
		return true
	default:
		return false
	}
}
