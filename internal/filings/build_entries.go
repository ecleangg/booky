package filings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/store"
	"github.com/ecleangg/booky/internal/support"
	"github.com/google/uuid"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

func (s *Service) buildEntries(ctx context.Context, facts []domain.AccountingFact, source entrySource) ([]domain.OSSUnionEntry, []domain.PeriodicSummaryEntry, []domain.FilingPeriod, error) {
	grouped := map[string][]domain.AccountingFact{}
	for _, fact := range facts {
		if !isFilingSourceGroup(fact.SourceGroupID) {
			continue
		}
		grouped[fact.SourceGroupID] = append(grouped[fact.SourceGroupID], fact)
	}

	groupIDs := make([]string, 0, len(grouped))
	for groupID := range grouped {
		groupIDs = append(groupIDs, groupID)
	}
	sort.Strings(groupIDs)

	var ossEntries []domain.OSSUnionEntry
	var psEntries []domain.PeriodicSummaryEntry
	periodMap := map[string]domain.FilingPeriod{}

	for _, groupID := range groupIDs {
		entryCtx, err := s.buildGroupContext(ctx, grouped[groupID], source)
		if err != nil {
			return nil, nil, nil, err
		}
		if entryCtx == nil {
			continue
		}

		if isOSSCandidate(*entryCtx) {
			entry, err := s.buildOSSUnionEntry(ctx, *entryCtx)
			if err != nil {
				return nil, nil, nil, err
			}
			if entry != nil {
				ossEntries = append(ossEntries, *entry)
				periodMap[periodKey(domain.FilingKindOSSUnion, entry.FilingPeriod)] = s.periodForKind(domain.FilingKindOSSUnion, entry.FilingPeriod)
			}
		}
		if isPSCandidate(*entryCtx) {
			entry, err := s.buildPeriodicSummaryEntry(ctx, *entryCtx)
			if err != nil {
				return nil, nil, nil, err
			}
			if entry != nil {
				psEntries = append(psEntries, *entry)
				periodMap[periodKey(domain.FilingKindPeriodicSummary, entry.FilingPeriod)] = s.periodForKind(domain.FilingKindPeriodicSummary, entry.FilingPeriod)
			}
		}
	}

	periods := make([]domain.FilingPeriod, 0, len(periodMap))
	for _, period := range periodMap {
		periods = append(periods, period)
	}
	sort.Slice(periods, func(i, j int) bool {
		if periods[i].Kind == periods[j].Kind {
			return periods[i].Period < periods[j].Period
		}
		return periods[i].Kind < periods[j].Kind
	})

	return ossEntries, psEntries, periods, nil
}

func (s *Service) buildGroupContext(ctx context.Context, facts []domain.AccountingFact, source entrySource) (*filingContext, error) {
	if len(facts) == 0 {
		return nil, nil
	}
	sort.Slice(facts, func(i, j int) bool {
		if facts[i].FactType == facts[j].FactType {
			return facts[i].CreatedAt.Before(facts[j].CreatedAt)
		}
		return facts[i].FactType < facts[j].FactType
	})

	groupFacts := summarizeFacts(facts)
	rep := groupFacts.Representative
	entryCtx := &filingContext{
		TaxCaseID:          rep.TaxCaseID,
		GroupID:            rep.SourceGroupID,
		SourceObjectType:   rep.SourceObjectType,
		SourceObjectID:     rep.SourceObjectID,
		StripeEventID:      rep.StripeEventID,
		PostingDate:        rep.PostingDate,
		OriginalSupplyDate: rep.PostingDate,
		MarketCode:         groupFacts.MarketCode,
		VATTreatment:       groupFacts.VATTreatment,
		RevenueSEKOre:      groupFacts.RevenueSEKOre,
		VATSEKOre:          groupFacts.VATSEKOre,
		SourceCurrency:     groupFacts.SourceCurrency,
		SourceAmountMinor:  groupFacts.SourceAmount,
		HasSourceAmount:    groupFacts.HasSourceMinor,
		ReviewReason:       groupFacts.ReviewReason,
	}

	if rep.TaxCaseID == nil {
		entryCtx.ReviewReason = "accounting facts are missing tax_case_id"
		return entryCtx, nil
	}
	taxCase, err := source.TaxCase(ctx, *rep.TaxCaseID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			entryCtx.ReviewReason = "tax case is missing"
			return entryCtx, nil
		}
		return nil, err
	}
	entryCtx.TaxStatus = strings.TrimSpace(valueOrEmpty(taxCase.TaxStatus))
	entryCtx.ReportabilityState = taxCase.ReportabilityState
	entryCtx.SaleType = strings.TrimSpace(valueOrEmpty(taxCase.SaleType))
	entryCtx.Country = strings.TrimSpace(valueOrEmpty(taxCase.Country))
	entryCtx.BuyerVATNumber = strings.TrimSpace(valueOrEmpty(taxCase.BuyerVATNumber))
	entryCtx.AmountSEKOre = groupFacts.GrossSEKOre
	if taxCase.SourceCurrency != nil {
		entryCtx.SourceCurrency = strings.ToUpper(strings.TrimSpace(*taxCase.SourceCurrency))
	}
	if taxCase.SourceAmountMinor != nil {
		entryCtx.SourceAmountMinor = *taxCase.SourceAmountMinor
		entryCtx.HasSourceAmount = true
	}
	if entryCtx.ReviewReason == "" {
		switch rep.SourceObjectType {
		case "charge":
			var charge chargePayload
			if err := json.Unmarshal(rep.Payload, &charge); err == nil && charge.Created > 0 {
				entryCtx.OriginalSupplyDate = time.Unix(charge.Created, 0).In(support.LocationOrUTC(s.Config))
			}
		case "refund":
			var refund refundPayload
			if err := json.Unmarshal(rep.Payload, &refund); err == nil {
				entryCtx.SourceCurrency = strings.ToUpper(refund.Currency)
				entryCtx.SourceAmountMinor = -refund.Amount
				entryCtx.HasSourceAmount = true
			}
		}
		if entryCtx.OriginalSupplyDate.IsZero() {
			entryCtx.OriginalSupplyDate = rep.PostingDate
		}
		if reason := filingUnsupportedReason(*entryCtx); reason != "" {
			entryCtx.ReviewReason = reason
		}
	}
	if rep.SourceObjectType == "refund" {
		entryCtx.AmountSEKOre = -groupFacts.GrossSEKOre
		entryCtx.RevenueSEKOre = -groupFacts.RevenueSEKOre
		entryCtx.VATSEKOre = -groupFacts.VATSEKOre
	}
	return entryCtx, nil
}

func (s *Service) buildOSSUnionEntry(ctx context.Context, input filingContext) (*domain.OSSUnionEntry, error) {
	filingPeriod := quarterPeriod(input.PostingDate)
	originalSupplyPeriod := quarterPeriod(input.OriginalSupplyDate)

	reviewState := domain.FilingReviewStateReady
	var reviewReason *string
	if strings.TrimSpace(input.ReviewReason) != "" {
		reviewState = domain.FilingReviewStateReview
		reason := strings.TrimSpace(input.ReviewReason)
		reviewReason = &reason
	}

	entry := &domain.OSSUnionEntry{
		ID:                   uuid.New(),
		BokioCompanyID:       s.Config.Bokio.CompanyID,
		TaxCaseID:            input.TaxCaseID,
		SourceGroupID:        input.GroupID,
		SourceObjectType:     input.SourceObjectType,
		SourceObjectID:       input.SourceObjectID,
		StripeEventID:        input.StripeEventID,
		OriginalSupplyPeriod: originalSupplyPeriod,
		FilingPeriod:         filingPeriod,
		ConsumptionCountry:   strings.ToUpper(strings.TrimSpace(input.Country)),
		OriginCountry:        strings.ToUpper(strings.TrimSpace(s.Config.Filings.OSSUnion.OriginCountry)),
		OriginIdentifier:     strings.ToUpper(strings.TrimSpace(s.Config.Filings.OSSUnion.OriginCountry)),
		SaleType:             input.SaleType,
		ReviewState:          reviewState,
		ReviewReason:         reviewReason,
	}
	if configuredRate, ok := configuredVATRateBasisPoints(s.Config, entry.ConsumptionCountry); ok {
		entry.VATRateBasisPoints = configuredRate
	} else {
		entry.VATRateBasisPoints = vatRateBasisPoints(input.RevenueSEKOre, input.VATSEKOre)
	}
	if reviewState == domain.FilingReviewStateReady {
		entry.TaxableAmountEURCents = input.RevenueSEKOre
		entry.VATAmountEURCents = input.VATSEKOre
	}
	if input.SourceObjectType == "refund" && filingPeriod != originalSupplyPeriod {
		entry.CorrectionTargetPeriod = &originalSupplyPeriod
	}
	payload, err := buildEntryPayload(input)
	if err != nil {
		return nil, err
	}
	entry.Payload = payload
	if reviewState == domain.FilingReviewStateReady && (!canEncodeLatin1(entry.OriginIdentifier) || !canEncodeLatin1(entry.ConsumptionCountry)) {
		reason := "OSS entry contains characters outside ISO-8859-1"
		entry.ReviewState = domain.FilingReviewStateReview
		entry.ReviewReason = &reason
		entry.TaxableAmountEURCents = 0
		entry.VATAmountEURCents = 0
	}
	return entry, nil
}

func (s *Service) buildPeriodicSummaryEntry(ctx context.Context, input filingContext) (*domain.PeriodicSummaryEntry, error) {
	filingPeriod := monthPeriod(input.PostingDate)

	reviewState := domain.FilingReviewStateReady
	var reviewReason *string
	if strings.TrimSpace(input.ReviewReason) != "" {
		reviewState = domain.FilingReviewStateReview
		reason := strings.TrimSpace(input.ReviewReason)
		reviewReason = &reason
	}

	rowType := mapPSSaleType(input.SaleType)
	if rowType == "" {
		reason := "periodic summary row type could not be resolved"
		reviewState = domain.FilingReviewStateReview
		reviewReason = &reason
	}

	amountSEKOre := input.AmountSEKOre
	if reviewState == domain.FilingReviewStateReady && amountSEKOre == 0 {
		reason := "periodic summary entry is missing settled SEK amount"
		reviewState = domain.FilingReviewStateReview
		reviewReason = &reason
	}
	if reviewState == domain.FilingReviewStateReview {
		amountSEKOre = 0
	}

	entry := &domain.PeriodicSummaryEntry{
		ID:                uuid.New(),
		BokioCompanyID:    s.Config.Bokio.CompanyID,
		TaxCaseID:         input.TaxCaseID,
		SourceGroupID:     input.GroupID,
		SourceObjectType:  input.SourceObjectType,
		SourceObjectID:    input.SourceObjectID,
		StripeEventID:     input.StripeEventID,
		FilingPeriod:      filingPeriod,
		BuyerVATNumber:    strings.ToUpper(strings.TrimSpace(input.BuyerVATNumber)),
		RowType:           rowType,
		AmountSEKOre:      amountSEKOre,
		ExportedAmountSEK: oreToWholeSEK(amountSEKOre),
		ReviewState:       reviewState,
		ReviewReason:      reviewReason,
	}
	payload, err := buildEntryPayload(input)
	if err != nil {
		return nil, err
	}
	entry.Payload = payload
	if reviewState == domain.FilingReviewStateReady && !canEncodeLatin1(entry.BuyerVATNumber) {
		reason := "periodic summary buyer VAT number contains characters outside ISO-8859-1"
		entry.ReviewState = domain.FilingReviewStateReview
		entry.ReviewReason = &reason
		entry.AmountSEKOre = 0
		entry.ExportedAmountSEK = 0
	}
	return entry, nil
}

func summarizeFacts(facts []domain.AccountingFact) filingFacts {
	out := filingFacts{Representative: facts[0]}
	for _, fact := range facts {
		if out.MarketCode == "" && fact.MarketCode != nil {
			out.MarketCode = strings.TrimSpace(*fact.MarketCode)
		}
		if out.VATTreatment == "" && fact.VATTreatment != nil {
			out.VATTreatment = strings.TrimSpace(*fact.VATTreatment)
		}
		if out.SourceCurrency == "" && fact.SourceCurrency != nil {
			out.SourceCurrency = strings.ToUpper(strings.TrimSpace(*fact.SourceCurrency))
		}
		if !out.HasSourceMinor && fact.SourceAmountMinor != nil {
			out.SourceAmount = *fact.SourceAmountMinor
			out.HasSourceMinor = true
		}
		if out.ReviewReason == "" && fact.ReviewReason != nil && strings.TrimSpace(*fact.ReviewReason) != "" {
			out.ReviewReason = strings.TrimSpace(*fact.ReviewReason)
		}

		switch fact.FactType {
		case "sale_receivable", "sale_review_receivable", "refund_receivable", "refund_review_receivable":
			out.GrossSEKOre = fact.AmountSEKOre
		case "sale_revenue", "refund_revenue":
			out.RevenueSEKOre = fact.AmountSEKOre
		case "sale_output_vat", "refund_output_vat":
			out.VATSEKOre = fact.AmountSEKOre
		}
	}
	return out
}

func buildEntryPayload(input filingContext) (json.RawMessage, error) {
	payload := map[string]any{
		"tax_case_id":          input.TaxCaseID,
		"tax_status":           input.TaxStatus,
		"reportability_state":  input.ReportabilityState,
		"source_group_id":      input.GroupID,
		"source_object_type":   input.SourceObjectType,
		"source_object_id":     input.SourceObjectID,
		"market_code":          input.MarketCode,
		"vat_treatment":        input.VATTreatment,
		"sale_category":        input.SaleCategory,
		"buyer_vat_number":     input.BuyerVATNumber,
		"country":              input.Country,
		"review_reason":        input.ReviewReason,
		"source_currency":      input.SourceCurrency,
		"source_amount_minor":  input.SourceAmountMinor,
		"amount_sek_ore":       input.AmountSEKOre,
		"revenue_sek_ore":      input.RevenueSEKOre,
		"vat_sek_ore":          input.VATSEKOre,
		"sale_type":            input.SaleType,
		"shipping_evidence":    input.ShippingEvidence,
		"original_supply_date": input.OriginalSupplyDate,
		"filing_posting_date":  input.PostingDate,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal filing payload: %w", err)
	}
	return b, nil
}

func oreToWholeSEK(amount int64) int64 {
	if amount >= 0 {
		return (amount + 50) / 100
	}
	return -(((-amount) + 50) / 100)
}

func canEncodeLatin1(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	_, _, err := transform.String(charmap.ISO8859_1.NewEncoder(), value)
	return err == nil
}
