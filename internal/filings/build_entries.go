package filings

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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

	switch rep.SourceObjectType {
	case "charge":
		charge, err := decodeCharge(rep.Payload)
		if err != nil {
			return nil, err
		}
		invoice, customer, err := loadChargeSupport(ctx, source, charge)
		if err != nil {
			return nil, err
		}
		entryCtx.Charge = &charge
		entryCtx.Invoice = invoice
		entryCtx.Customer = customer
		entryCtx.AllMetadata = support.MergeStringMaps(metadataFromCustomer(customer), metadataFromInvoice(invoice), charge.Metadata)
		entryCtx.SaleCategory = support.MapString(entryCtx.AllMetadata, "booky_sale_category", "sale_category")
		entryCtx.BuyerVATNumber = resolveCustomerVATID(entryCtx.AllMetadata, invoice)
		entryCtx.Country = resolveCountryEvidence(charge, invoice, customer, entryCtx.SaleCategory, entryCtx.AllMetadata)
		entryCtx.ShippingEvidence = hasShippingEvidence(invoice, customer)
		entryCtx.AmountSEKOre = groupFacts.GrossSEKOre
		entryCtx.OriginalSupplyDate = time.Unix(charge.Created, 0).In(support.LocationOrUTC(s.Config))
	case "refund":
		refund, err := decodeRefund(rep.Payload)
		if err != nil {
			return nil, err
		}
		parentRaw, err := source.ParentChargeForRefund(ctx, refund.ID, rep.StripeEventID)
		if err != nil {
			return nil, err
		}
		charge, err := decodeCharge(parentRaw)
		if err != nil {
			return nil, err
		}
		invoice, customer, err := loadChargeSupport(ctx, source, charge)
		if err != nil {
			return nil, err
		}
		entryCtx.Refund = &refund
		entryCtx.Charge = &charge
		entryCtx.Invoice = invoice
		entryCtx.Customer = customer
		entryCtx.AllMetadata = support.MergeStringMaps(metadataFromCustomer(customer), metadataFromInvoice(invoice), charge.Metadata, refund.Metadata)
		entryCtx.SaleCategory = support.MapString(entryCtx.AllMetadata, "booky_sale_category", "sale_category")
		entryCtx.BuyerVATNumber = resolveCustomerVATID(entryCtx.AllMetadata, invoice)
		entryCtx.Country = resolveCountryEvidence(charge, invoice, customer, entryCtx.SaleCategory, entryCtx.AllMetadata)
		entryCtx.ShippingEvidence = hasShippingEvidence(invoice, customer)
		entryCtx.AmountSEKOre = -groupFacts.GrossSEKOre
		entryCtx.RevenueSEKOre = -groupFacts.RevenueSEKOre
		entryCtx.VATSEKOre = -groupFacts.VATSEKOre
		entryCtx.SourceCurrency = strings.ToUpper(refund.Currency)
		entryCtx.SourceAmountMinor = -refund.Amount
		entryCtx.HasSourceAmount = true
		entryCtx.OriginalSupplyDate = time.Unix(charge.Created, 0).In(support.LocationOrUTC(s.Config))
	default:
		return nil, nil
	}

	if strings.TrimSpace(entryCtx.Country) == "" && len(strings.TrimSpace(entryCtx.MarketCode)) == 2 {
		entryCtx.Country = strings.ToUpper(strings.TrimSpace(entryCtx.MarketCode))
	}
	saleType, reviewReason := resolveSaleType(entryCtx.SaleCategory, entryCtx.ShippingEvidence)
	entryCtx.SaleType = saleType
	if entryCtx.ReviewReason == "" && reviewReason != "" {
		entryCtx.ReviewReason = reviewReason
	}
	if entryCtx.ReviewReason == "" {
		if reason := filingUnsupportedReason(*entryCtx); reason != "" {
			entryCtx.ReviewReason = reason
		}
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

	rate, err := s.Rates.OSSPeriodEndEURSEK(ctx, filingPeriod)
	if err != nil {
		return nil, err
	}

	entry := &domain.OSSUnionEntry{
		ID:                   uuid.New(),
		BokioCompanyID:       s.Config.Bokio.CompanyID,
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
		VATRateBasisPoints:   vatRateBasisPoints(input.RevenueSEKOre, input.VATSEKOre),
		ReviewState:          reviewState,
		ReviewReason:         reviewReason,
	}
	if reviewState == domain.FilingReviewStateReady {
		entry.TaxableAmountEURCents = int64(math.Round(float64(input.RevenueSEKOre) / rate.Rate))
		entry.VATAmountEURCents = int64(math.Round(float64(input.VATSEKOre) / rate.Rate))
	}
	if input.SourceObjectType == "refund" && filingPeriod != originalSupplyPeriod {
		entry.CorrectionTargetPeriod = &originalSupplyPeriod
	}
	payload, err := buildEntryPayload(input, rate)
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
	var rate domain.FXRate
	if reviewState == domain.FilingReviewStateReady {
		if strings.ToUpper(strings.TrimSpace(input.SourceCurrency)) == "SEK" {
			rate = domain.FXRate{
				Provider:      "identity",
				BaseCurrency:  "SEK",
				QuoteCurrency: "SEK",
				Period:        filingPeriod,
				Rate:          1,
			}
		} else if !input.HasSourceAmount {
			reason := "periodic summary entry is missing source amount for FX conversion"
			reviewState = domain.FilingReviewStateReview
			reviewReason = &reason
		} else {
			var err error
			rate, err = s.Rates.PSMonthlyAverage(ctx, strings.ToUpper(strings.TrimSpace(input.SourceCurrency)), filingPeriod)
			if err != nil {
				return nil, err
			}
			amountSEKOre = int64(math.Round(float64(input.SourceAmountMinor) * rate.Rate))
		}
	}
	if reviewState == domain.FilingReviewStateReview {
		amountSEKOre = 0
	}

	entry := &domain.PeriodicSummaryEntry{
		ID:                uuid.New(),
		BokioCompanyID:    s.Config.Bokio.CompanyID,
		SourceGroupID:     input.GroupID,
		SourceObjectType:  input.SourceObjectType,
		SourceObjectID:    input.SourceObjectID,
		StripeEventID:     input.StripeEventID,
		FilingPeriod:      filingPeriod,
		BuyerVATNumber:    strings.ToUpper(strings.TrimSpace(input.BuyerVATNumber)),
		RowType:           rowType,
		AmountSEKOre:      amountSEKOre,
		ExportedAmountSEK: int64(math.Round(float64(amountSEKOre) / 100.0)),
		ReviewState:       reviewState,
		ReviewReason:      reviewReason,
	}
	payload, err := buildEntryPayload(input, rate)
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

func loadChargeSupport(ctx context.Context, source entrySource, charge chargePayload) (*invoicePayload, *customerPayload, error) {
	var invoice *invoicePayload
	if strings.TrimSpace(charge.Invoice) != "" {
		raw, err := source.Snapshot(ctx, "invoice", strings.TrimSpace(charge.Invoice))
		if err != nil && err != store.ErrNotFound {
			return nil, nil, err
		}
		if len(raw) > 0 {
			decoded, err := decodeInvoice(raw)
			if err != nil {
				return nil, nil, err
			}
			invoice = &decoded
		}
	}

	customerID := strings.TrimSpace(charge.Customer)
	if customerID == "" && invoice != nil {
		customerID = strings.TrimSpace(invoice.Customer)
	}
	var customer *customerPayload
	if customerID != "" {
		raw, err := source.Snapshot(ctx, "customer", customerID)
		if err != nil && err != store.ErrNotFound {
			return nil, nil, err
		}
		if len(raw) > 0 {
			decoded, err := decodeCustomer(raw)
			if err != nil {
				return nil, nil, err
			}
			customer = &decoded
		}
	}
	return invoice, customer, nil
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

func buildEntryPayload(input filingContext, rate domain.FXRate) (json.RawMessage, error) {
	payload := map[string]any{
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
		"fx_provider":          rate.Provider,
		"fx_rate":              rate.Rate,
		"fx_observed_at":       rate.ObservedAt,
		"original_supply_date": input.OriginalSupplyDate,
		"filing_posting_date":  input.PostingDate,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal filing payload: %w", err)
	}
	return b, nil
}

func canEncodeLatin1(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	_, _, err := transform.String(charmap.ISO8859_1.NewEncoder(), value)
	return err == nil
}
