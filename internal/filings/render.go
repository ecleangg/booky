package filings

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/domain"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

type RenderedFile struct {
	Filename string
	Content  []byte
}

func RenderOSSUnion(cfg config.Config, period string, entries []domain.OSSUnionEntry) (RenderedFile, error) {
	type lineKey struct {
		Origin   string
		Country  string
		Rate     int
		SaleType string
	}
	type correctionKey struct {
		Target  string
		Country string
	}

	mainLines := map[lineKey]struct {
		Taxable int64
		VAT     int64
	}{}
	corrections := map[correctionKey]int64{}
	for _, entry := range entries {
		if entry.ReviewState != domain.FilingReviewStateReady {
			continue
		}
		if entry.CorrectionTargetPeriod != nil {
			key := correctionKey{Target: *entry.CorrectionTargetPeriod, Country: entry.ConsumptionCountry}
			corrections[key] += entry.VATAmountEURCents
			continue
		}
		key := lineKey{
			Origin:   entry.OriginIdentifier,
			Country:  entry.ConsumptionCountry,
			Rate:     entry.VATRateBasisPoints,
			SaleType: entry.SaleType,
		}
		line := mainLines[key]
		line.Taxable += entry.TaxableAmountEURCents
		line.VAT += entry.VATAmountEURCents
		mainLines[key] = line
	}

	var rows []string
	rows = append(rows, "OSS_001;")
	year, quarter, err := ossQuarterParts(period)
	if err != nil {
		return RenderedFile{}, err
	}
	rows = append(rows, fmt.Sprintf("%s;%d;%04d;", strings.TrimSpace(cfg.Filings.OSSUnion.IdentifierNumber), quarter, year))

	mainKeys := make([]lineKey, 0, len(mainLines))
	for key := range mainLines {
		mainKeys = append(mainKeys, key)
	}
	sort.Slice(mainKeys, func(i, j int) bool {
		if mainKeys[i].Origin == mainKeys[j].Origin {
			if mainKeys[i].Country == mainKeys[j].Country {
				if mainKeys[i].Rate == mainKeys[j].Rate {
					return mainKeys[i].SaleType < mainKeys[j].SaleType
				}
				return mainKeys[i].Rate < mainKeys[j].Rate
			}
			return mainKeys[i].Country < mainKeys[j].Country
		}
		return mainKeys[i].Origin < mainKeys[j].Origin
	})
	for _, key := range mainKeys {
		line := mainLines[key]
		if line.Taxable == 0 && line.VAT == 0 {
			continue
		}
		rows = append(rows, fmt.Sprintf("%s;%s;%s;%s;%s;%s;",
			key.Origin,
			key.Country,
			formatDecimalBasisPoints(key.Rate),
			formatDecimalCents(line.Taxable),
			formatDecimalCents(line.VAT),
			key.SaleType,
		))
	}

	correctionKeys := make([]correctionKey, 0, len(corrections))
	for key := range corrections {
		correctionKeys = append(correctionKeys, key)
	}
	sort.Slice(correctionKeys, func(i, j int) bool {
		if correctionKeys[i].Target == correctionKeys[j].Target {
			return correctionKeys[i].Country < correctionKeys[j].Country
		}
		return correctionKeys[i].Target < correctionKeys[j].Target
	})
	if len(correctionKeys) > 0 {
		rows = append(rows, "CORRECTIONS")
		for _, key := range correctionKeys {
			if corrections[key] == 0 {
				continue
			}
			rows = append(rows, fmt.Sprintf("%s;%s;%s;", compactQuarterPeriod(key.Target), key.Country, formatDecimalCents(corrections[key])))
		}
	}

	content, err := encodeISO88591(strings.Join(rows, "\r\n") + "\r\n")
	if err != nil {
		return RenderedFile{}, err
	}
	return RenderedFile{
		Filename: fmt.Sprintf("oss-union-%s.txt", period),
		Content:  content,
	}, nil
}

func RenderPeriodicSummary(cfg config.Config, period string, entries []domain.PeriodicSummaryEntry) (RenderedFile, error) {
	type totals struct {
		Goods       int64
		Services    int64
		HasGoods    bool
		HasServices bool
	}

	grouped := map[string]totals{}
	for _, entry := range entries {
		if entry.ReviewState != domain.FilingReviewStateReady {
			continue
		}
		total := grouped[entry.BuyerVATNumber]
		switch entry.RowType {
		case "goods":
			total.Goods += entry.ExportedAmountSEK
			total.HasGoods = true
		case "services":
			total.Services += entry.ExportedAmountSEK
			total.HasServices = true
		}
		grouped[entry.BuyerVATNumber] = total
	}

	rows := []string{
		"SKV574008;",
		fmt.Sprintf("%s;%s;%s;%s;%s",
			strings.TrimSpace(cfg.Filings.PeriodicSummary.ReportingVATNumber),
			psPeriodCode(period),
			strings.TrimSpace(cfg.Filings.PeriodicSummary.ResponsibleName),
			strings.TrimSpace(cfg.Filings.PeriodicSummary.ResponsiblePhone),
			strings.TrimSpace(cfg.Filings.PeriodicSummary.ResponsibleEmail),
		),
	}

	buyers := make([]string, 0, len(grouped))
	for buyer := range grouped {
		buyers = append(buyers, buyer)
	}
	sort.Strings(buyers)
	for _, buyer := range buyers {
		total := grouped[buyer]
		if !total.HasGoods && !total.HasServices {
			continue
		}
		rows = append(rows, fmt.Sprintf("%s;%s;;%s;",
			buyer,
			psAmountField(total.Goods, total.HasGoods),
			psAmountField(total.Services, total.HasServices),
		))
	}

	content, err := encodeISO88591(strings.Join(rows, "\r\n") + "\r\n")
	if err != nil {
		return RenderedFile{}, err
	}
	return RenderedFile{
		Filename: fmt.Sprintf("periodisk-sammanstallning-%s.txt", period),
		Content:  content,
	}, nil
}

func ossQuarterParts(period string) (int, int, error) {
	var year, quarter int
	if _, err := fmt.Sscanf(period, "%4d-Q%d", &year, &quarter); err != nil || quarter < 1 || quarter > 4 {
		return 0, 0, fmt.Errorf("invalid quarter period %q", period)
	}
	return year, quarter, nil
}

func compactQuarterPeriod(period string) string {
	year, quarter, err := ossQuarterParts(period)
	if err != nil {
		return period
	}
	return fmt.Sprintf("%04dQ%d", year, quarter)
}

func psPeriodCode(period string) string {
	if len(period) != len("2006-01") {
		return period
	}
	return period[2:4] + period[5:7]
}

func formatDecimalBasisPoints(bps int) string {
	return strings.ReplaceAll(fmt.Sprintf("%.2f", float64(bps)/100.0), ".", ",")
}

func formatDecimalCents(cents int64) string {
	sign := ""
	value := cents
	if value < 0 {
		sign = "-"
		value = -value
	}
	return fmt.Sprintf("%s%d,%02d", sign, value/100, value%100)
}

func psAmountField(value int64, present bool) string {
	if !present {
		return ""
	}
	return fmt.Sprintf("%d", value)
}

func encodeISO88591(text string) ([]byte, error) {
	encoded, _, err := transform.String(charmap.ISO8859_1.NewEncoder(), text)
	if err != nil {
		return nil, fmt.Errorf("encode ISO-8859-1: %w", err)
	}
	return []byte(encoded), nil
}
