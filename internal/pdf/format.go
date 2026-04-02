package pdf

import (
	"fmt"
	"math"
	"strings"

	"github.com/ecleangg/booky/internal/domain"
)

func formatSEK(ore int64) string {
	return fmt.Sprintf("%.2f", float64(ore)/100.0)
}

func formatSummaryValue(key string, value any) string {
	if strings.HasSuffix(key, "_ore") {
		switch v := value.(type) {
		case int64:
			return fmt.Sprintf("%s SEK", formatSEK(v))
		case int:
			return fmt.Sprintf("%s SEK", formatSEK(int64(v)))
		case float64:
			return fmt.Sprintf("%s SEK", formatSEK(int64(v)))
		}
	}
	return fmt.Sprintf("%v", value)
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func summaryString(summary map[string]any, key string) string {
	if summary == nil {
		return ""
	}
	value, ok := summary[key]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%v", value)
}

func sourceObjectLabel(fact domain.AccountingFact) string {
	if fact.SourceObjectType == "" && fact.SourceObjectID == "" {
		return ""
	}
	if fact.SourceObjectType == "" {
		return fact.SourceObjectID
	}
	if fact.SourceObjectID == "" {
		return fact.SourceObjectType
	}
	return fact.SourceObjectType + ":" + fact.SourceObjectID
}

func sourceAmountLabel(fact domain.AccountingFact) string {
	if fact.SourceAmountMinor == nil && fact.SourceCurrency == nil {
		return ""
	}
	if fact.SourceAmountMinor == nil {
		return valueOrEmpty(fact.SourceCurrency)
	}
	currency := valueOrEmpty(fact.SourceCurrency)
	if currency == "" {
		return fmt.Sprintf("%d", *fact.SourceAmountMinor)
	}
	return fmt.Sprintf("%s %s", formatMinorAmount(*fact.SourceAmountMinor, currency), currency)
}

func formatMinorAmount(amountMinor int64, currency string) string {
	exponent := currencyExponent(currency)
	divisor := math.Pow10(exponent)
	return fmt.Sprintf("%.*f", exponent, float64(amountMinor)/divisor)
}

func currencyExponent(currency string) int {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "BIF", "CLP", "DJF", "GNF", "JPY", "KMF", "KRW", "MGA", "PYG", "RWF", "UGX", "VND", "VUV", "XAF", "XOF", "XPF":
		return 0
	default:
		return 2
	}
}
