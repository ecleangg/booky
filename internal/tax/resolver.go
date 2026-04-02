package tax

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/support"
	"github.com/google/uuid"
)

type EvidenceDossier struct {
	Root struct {
		ObjectType string `json:"object_type"`
		ObjectID   string `json:"object_id"`
	} `json:"root"`
	Status             *string                    `json:"status,omitempty"`
	ReportabilityState string                     `json:"reportability_state"`
	ReviewReason       *string                    `json:"review_reason,omitempty"`
	SaleType           *string                    `json:"sale_type,omitempty"`
	Country            *string                    `json:"country,omitempty"`
	CountrySource      *string                    `json:"country_source,omitempty"`
	BuyerVATNumber     *string                    `json:"buyer_vat_number,omitempty"`
	BuyerVATVerified   bool                       `json:"buyer_vat_verified"`
	BuyerIsBusiness    bool                       `json:"buyer_is_business"`
	AutomaticTax       map[string]any             `json:"automatic_tax,omitempty"`
	StripeTax          map[string]any             `json:"stripe_tax,omitempty"`
	InvoicePDFURL      *string                    `json:"invoice_pdf_url,omitempty"`
	Corroborating      map[string]any             `json:"corroborating,omitempty"`
	Objects            []domain.TaxCaseObject     `json:"objects"`
	ManualEvidence     []domain.ManualTaxEvidence `json:"manual_evidence,omitempty"`
}

type BuildResult struct {
	Case    domain.TaxCase
	Objects []domain.TaxCaseObject
	Dossier EvidenceDossier
}

func BuildCase(companyID uuid.UUID, livemode bool, snapshots []domain.ObjectSnapshot, manual []domain.ManualTaxEvidence, existingID *uuid.UUID) (BuildResult, error) {
	index := snapshotIndexFrom(snapshots)
	rootType, rootID := resolveRoot(index)
	objects := buildObjectLinks(existingID, rootType, rootID, snapshots)

	metadata := mergedMetadata(index)
	saleType, saleTypeReason := resolveSaleType(index, metadata, manual)
	country, countrySource := resolveCountry(index, saleType, manual)
	vatNumber, vatVerified := resolveBuyerVAT(index, manual)
	customerTaxExempt := resolveCustomerTaxExempt(index)
	automaticTaxEnabled, automaticTaxStatus := resolveAutomaticTax(index)
	taxKnown, taxAmountMinor, reverseCharge, zeroRated, reasons := resolveStripeTax(index)
	business := resolveBuyerIsBusiness(vatNumber, vatVerified, reverseCharge, customerTaxExempt, manual)

	reportability, reviewReason := resolveReportability(country, saleType, business, vatNumber, vatVerified, taxKnown, manual)
	status := classifyStatus(country, business)

	if reportability == domain.NeedsReview && reviewReason == "" && saleTypeReason != "" {
		reviewReason = saleTypeReason
	}

	dossier := EvidenceDossier{
		Status:             status,
		ReportabilityState: reportability,
		Objects:            objects,
		ManualEvidence:     manual,
		BuyerVATNumber:     vatNumber,
		BuyerVATVerified:   vatVerified,
		BuyerIsBusiness:    business,
		SaleType:           saleType,
		Country:            country,
		CountrySource:      countrySource,
		InvoicePDFURL:      resolveInvoicePDF(index),
		AutomaticTax: map[string]any{
			"enabled": automaticTaxEnabled,
			"status":  valueOrEmpty(automaticTaxStatus),
		},
		StripeTax: map[string]any{
			"amount_known":       taxKnown,
			"amount_minor":       int64OrNil(taxAmountMinor),
			"reverse_charge":     reverseCharge,
			"zero_rated":         zeroRated,
			"taxability_reasons": reasons,
		},
		Corroborating: map[string]any{
			"payment_method_country": resolvePaymentMethodCountry(index),
			"customer_tax_exempt":    customerTaxExempt,
		},
		ReviewReason: reviewPtr(reviewReason),
	}
	dossier.Root.ObjectType = rootType
	dossier.Root.ObjectID = rootID

	dossierJSON, err := json.Marshal(dossier)
	if err != nil {
		return BuildResult{}, err
	}

	caseID := uuid.New()
	if existingID != nil && *existingID != uuid.Nil {
		caseID = *existingID
	}
	for i := range objects {
		objects[i].TaxCaseID = caseID
	}

	result := domain.TaxCase{
		ID:                     caseID,
		BokioCompanyID:         companyID,
		RootObjectType:         rootType,
		RootObjectID:           rootID,
		Livemode:               livemode,
		SourceCurrency:         resolveSourceCurrency(index),
		SourceAmountMinor:      resolveSourceAmount(index),
		SaleType:               saleType,
		Country:                country,
		CountrySource:          countrySource,
		BuyerVATNumber:         vatNumber,
		BuyerVATVerified:       vatVerified,
		BuyerIsBusiness:        business,
		TaxStatus:              status,
		ReportabilityState:     reportability,
		ReviewReason:           reviewPtr(reviewReason),
		AutomaticTaxEnabled:    automaticTaxEnabled,
		AutomaticTaxStatus:     automaticTaxStatus,
		StripeTaxAmountKnown:   taxKnown,
		StripeTaxAmountMinor:   taxAmountMinor,
		StripeTaxReverseCharge: reverseCharge,
		StripeTaxZeroRated:     zeroRated,
		InvoicePDFURL:          resolveInvoicePDF(index),
		Dossier:                dossierJSON,
	}
	return BuildResult{Case: result, Objects: objects, Dossier: dossier}, nil
}

type snapshotIndex struct {
	invoice         *invoice
	checkoutSession *checkoutSession
	paymentIntent   *paymentIntent
	charge          *charge
	refund          *refund
	customer        *customer
	taxIDs          []taxID
}

func snapshotIndexFrom(snapshots []domain.ObjectSnapshot) snapshotIndex {
	var index snapshotIndex
	for _, snapshot := range snapshots {
		switch snapshot.ObjectType {
		case "invoice":
			if index.invoice == nil {
				var payload invoice
				if json.Unmarshal(snapshot.Payload, &payload) == nil {
					index.invoice = &payload
				}
			}
		case "checkout_session":
			if index.checkoutSession == nil {
				var payload checkoutSession
				if json.Unmarshal(snapshot.Payload, &payload) == nil {
					index.checkoutSession = &payload
				}
			}
		case "payment_intent":
			if index.paymentIntent == nil {
				var payload paymentIntent
				if json.Unmarshal(snapshot.Payload, &payload) == nil {
					index.paymentIntent = &payload
				}
			}
		case "charge":
			if index.charge == nil {
				var payload charge
				if json.Unmarshal(snapshot.Payload, &payload) == nil {
					index.charge = &payload
				}
			}
		case "refund":
			if index.refund == nil {
				var payload refund
				if json.Unmarshal(snapshot.Payload, &payload) == nil {
					index.refund = &payload
				}
			}
		case "customer":
			if index.customer == nil {
				var payload customer
				if json.Unmarshal(snapshot.Payload, &payload) == nil {
					index.customer = &payload
				}
			}
		case "tax_id":
			var payload taxID
			if json.Unmarshal(snapshot.Payload, &payload) == nil {
				index.taxIDs = append(index.taxIDs, payload)
			}
		}
	}
	return index
}

func resolveRoot(index snapshotIndex) (string, string) {
	switch {
	case index.invoice != nil && strings.TrimSpace(index.invoice.ID) != "":
		return "invoice", strings.TrimSpace(index.invoice.ID)
	case index.checkoutSession != nil && strings.TrimSpace(index.checkoutSession.ID) != "":
		return "checkout_session", strings.TrimSpace(index.checkoutSession.ID)
	case index.paymentIntent != nil && strings.TrimSpace(index.paymentIntent.ID) != "":
		return "payment_intent", strings.TrimSpace(index.paymentIntent.ID)
	case index.charge != nil && strings.TrimSpace(index.charge.ID) != "":
		return "charge", strings.TrimSpace(index.charge.ID)
	default:
		return "unknown", "unknown"
	}
}

func buildObjectLinks(existingID *uuid.UUID, rootType, rootID string, snapshots []domain.ObjectSnapshot) []domain.TaxCaseObject {
	out := make([]domain.TaxCaseObject, 0, len(snapshots))
	seen := map[string]struct{}{}
	caseID := uuid.Nil
	if existingID != nil {
		caseID = *existingID
	}
	for _, snapshot := range snapshots {
		key := snapshot.ObjectType + ":" + snapshot.ObjectID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		role := "supporting"
		if snapshot.ObjectType == rootType && snapshot.ObjectID == rootID {
			role = "root"
		}
		out = append(out, domain.TaxCaseObject{
			TaxCaseID:  caseID,
			ObjectType: snapshot.ObjectType,
			ObjectID:   snapshot.ObjectID,
			ObjectRole: role,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ObjectRole == out[j].ObjectRole {
			if out[i].ObjectType == out[j].ObjectType {
				return out[i].ObjectID < out[j].ObjectID
			}
			return out[i].ObjectType < out[j].ObjectType
		}
		return out[i].ObjectRole < out[j].ObjectRole
	})
	return out
}

func mergedMetadata(index snapshotIndex) map[string]string {
	var invoiceMetadata, sessionMetadata, piMetadata, chargeMetadata, customerMetadata map[string]string
	if index.invoice != nil {
		invoiceMetadata = index.invoice.Metadata
	}
	if index.checkoutSession != nil {
		sessionMetadata = index.checkoutSession.Metadata
	}
	if index.paymentIntent != nil {
		piMetadata = index.paymentIntent.Metadata
	}
	if index.charge != nil {
		chargeMetadata = index.charge.Metadata
	}
	if index.customer != nil {
		customerMetadata = index.customer.Metadata
	}
	return support.MergeStringMaps(customerMetadata, chargeMetadata, piMetadata, sessionMetadata, invoiceMetadata)
}

func resolveSaleType(index snapshotIndex, metadata map[string]string, manual []domain.ManualTaxEvidence) (*string, string) {
	for _, ev := range manual {
		if ev.SaleType != nil && strings.TrimSpace(*ev.SaleType) != "" {
			value := normalizeSaleType(*ev.SaleType)
			if value != "" {
				return &value, ""
			}
		}
	}
	if value := resolveStripeSaleType(index); value != "" {
		return &value, ""
	}
	if raw := support.MapString(metadata, "booky_sale_category", "sale_category"); raw != "" {
		value := normalizeSaleType(raw)
		if value != "" {
			return &value, ""
		}
	}
	if hasShippingEvidence(index) {
		value := "GOODS"
		return &value, ""
	}
	return nil, "sale type could not be resolved"
}

func resolveStripeSaleType(index snapshotIndex) string {
	if value := resolveInvoiceLineSaleType(index.invoice); value != "" {
		return value
	}
	if value := resolveCheckoutLineSaleType(index.checkoutSession); value != "" {
		return value
	}
	return ""
}

func resolveInvoiceLineSaleType(invoice *invoice) string {
	if invoice == nil {
		return ""
	}
	for _, line := range invoice.Lines.Data {
		if line.Price != nil {
			if value := saleTypeFromProduct(line.Price.Product); value != "" {
				return value
			}
		}
		if line.Pricing != nil && line.Pricing.PriceDetails != nil {
			if value := saleTypeFromProduct(line.Pricing.PriceDetails.Product); value != "" {
				return value
			}
		}
	}
	return ""
}

func resolveCheckoutLineSaleType(session *checkoutSession) string {
	if session == nil {
		return ""
	}
	for _, line := range session.LineItems.Data {
		if line.Price == nil {
			continue
		}
		if value := saleTypeFromProduct(line.Price.Product); value != "" {
			return value
		}
	}
	return ""
}

func saleTypeFromProduct(product productDetails) string {
	switch strings.ToLower(strings.TrimSpace(product.Type)) {
	case "service":
		return "SERVICES"
	case "good":
		return "GOODS"
	}
	if product.Shippable != nil && *product.Shippable {
		return "GOODS"
	}
	switch strings.TrimSpace(product.TaxCode) {
	case "txcd_10103000":
		return "SERVICES"
	}
	return normalizeSaleType(support.MapString(product.Metadata, "booky_sale_category", "sale_category"))
}

func normalizeSaleType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "goods", "physical", "physical_goods":
		return "GOODS"
	case "services", "service", "digital_services", "digital":
		return "SERVICES"
	default:
		return ""
	}
}

func hasShippingEvidence(index snapshotIndex) bool {
	return invoiceShippingCountry(index.invoice) != "" ||
		checkoutShippingCountry(index.checkoutSession) != "" ||
		customerShippingCountry(index.customer) != ""
}

func resolveCountry(index snapshotIndex, saleType *string, manual []domain.ManualTaxEvidence) (*string, *string) {
	preferShipping := saleType != nil && *saleType == "GOODS"
	if country, source := invoiceCountry(index.invoice, preferShipping); country != "" {
		return strPtr(country), strPtr(source)
	}
	if country, source := checkoutCountry(index.checkoutSession, preferShipping); country != "" {
		return strPtr(country), strPtr(source)
	}
	if country, source := customerCountryWithSource(index.customer, preferShipping); country != "" {
		return strPtr(country), strPtr(source)
	}
	if country, source := chargeCountry(index.charge); country != "" {
		return strPtr(country), strPtr(source)
	}
	for _, ev := range manual {
		if ev.Country != nil && strings.TrimSpace(*ev.Country) != "" {
			country := support.NormalizeCountry(*ev.Country)
			source := "manual_tax_evidence"
			if ev.CountrySource != nil && strings.TrimSpace(*ev.CountrySource) != "" {
				source = strings.TrimSpace(*ev.CountrySource)
			}
			return &country, &source
		}
	}
	return nil, nil
}

func invoiceCountry(invoice *invoice, preferShipping bool) (string, string) {
	if invoice == nil {
		return "", ""
	}
	if preferShipping {
		if country := invoiceShippingCountry(invoice); country != "" {
			return country, "invoice.customer_shipping"
		}
	}
	if invoice.CustomerAddress != nil && strings.TrimSpace(invoice.CustomerAddress.Country) != "" {
		return support.NormalizeCountry(invoice.CustomerAddress.Country), "invoice.customer_address"
	}
	if !preferShipping {
		if country := invoiceShippingCountry(invoice); country != "" {
			return country, "invoice.customer_shipping"
		}
	}
	return "", ""
}

func invoiceShippingCountry(invoice *invoice) string {
	if invoice == nil || invoice.CustomerShipping == nil {
		return ""
	}
	return support.NormalizeCountry(invoice.CustomerShipping.Address.Country)
}

func checkoutCountry(session *checkoutSession, preferShipping bool) (string, string) {
	if session == nil {
		return "", ""
	}
	if preferShipping && session.ShippingDetails != nil && strings.TrimSpace(session.ShippingDetails.Address.Country) != "" {
		return support.NormalizeCountry(session.ShippingDetails.Address.Country), "checkout.shipping_details"
	}
	if session.CustomerDetails != nil && strings.TrimSpace(session.CustomerDetails.Address.Country) != "" {
		return support.NormalizeCountry(session.CustomerDetails.Address.Country), "checkout.customer_details"
	}
	if !preferShipping && session.ShippingDetails != nil && strings.TrimSpace(session.ShippingDetails.Address.Country) != "" {
		return support.NormalizeCountry(session.ShippingDetails.Address.Country), "checkout.shipping_details"
	}
	return "", ""
}

func checkoutShippingCountry(session *checkoutSession) string {
	if session == nil || session.ShippingDetails == nil {
		return ""
	}
	return support.NormalizeCountry(session.ShippingDetails.Address.Country)
}

func customerCountryWithSource(customer *customer, preferShipping bool) (string, string) {
	if customer == nil {
		return "", ""
	}
	if preferShipping {
		if country := customerShippingCountry(customer); country != "" {
			return country, "customer.shipping"
		}
	}
	if customer.Address != nil && strings.TrimSpace(customer.Address.Country) != "" {
		return support.NormalizeCountry(customer.Address.Country), "customer.address"
	}
	if !preferShipping {
		if country := customerShippingCountry(customer); country != "" {
			return country, "customer.shipping"
		}
	}
	return "", ""
}

func customerShippingCountry(customer *customer) string {
	if customer == nil || customer.Shipping == nil {
		return ""
	}
	return support.NormalizeCountry(customer.Shipping.Address.Country)
}

func chargeCountry(charge *charge) (string, string) {
	if charge == nil {
		return "", ""
	}
	if charge.CustomerDetails != nil && strings.TrimSpace(charge.CustomerDetails.Address.Country) != "" {
		return support.NormalizeCountry(charge.CustomerDetails.Address.Country), "charge.customer_details"
	}
	if strings.TrimSpace(charge.BillingDetails.Address.Country) != "" {
		return support.NormalizeCountry(charge.BillingDetails.Address.Country), "charge.billing_details"
	}
	return "", ""
}

func resolveBuyerVAT(index snapshotIndex, manual []domain.ManualTaxEvidence) (*string, bool) {
	if index.invoice != nil {
		for _, item := range index.invoice.CustomerTaxIDs {
			if strings.TrimSpace(item.Value) == "" {
				continue
			}
			value := strings.ToUpper(strings.TrimSpace(item.Value))
			return &value, hasVerifiedTaxID(index.taxIDs, value)
		}
	}
	if index.checkoutSession != nil && index.checkoutSession.CustomerDetails != nil {
		for _, item := range index.checkoutSession.CustomerDetails.TaxIDs {
			if strings.TrimSpace(item.Value) == "" {
				continue
			}
			value := strings.ToUpper(strings.TrimSpace(item.Value))
			return &value, taxIDVerified(item)
		}
	}
	for _, item := range index.taxIDs {
		if strings.TrimSpace(item.Value) == "" {
			continue
		}
		value := strings.ToUpper(strings.TrimSpace(item.Value))
		return &value, taxIDVerified(item)
	}
	for _, ev := range manual {
		if ev.BuyerVATNumber != nil && strings.TrimSpace(*ev.BuyerVATNumber) != "" {
			value := strings.ToUpper(strings.TrimSpace(*ev.BuyerVATNumber))
			return &value, ev.BuyerVATVerified != nil && *ev.BuyerVATVerified
		}
	}
	return nil, false
}

func hasVerifiedTaxID(items []taxID, wanted string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Value), strings.TrimSpace(wanted)) && taxIDVerified(item) {
			return true
		}
	}
	return false
}

func taxIDVerified(item taxID) bool {
	return item.Verification != nil && strings.EqualFold(strings.TrimSpace(item.Verification.Status), "verified")
}

func resolveCustomerTaxExempt(index snapshotIndex) string {
	switch {
	case index.invoice != nil && strings.TrimSpace(index.invoice.CustomerTaxExempt) != "":
		return strings.TrimSpace(index.invoice.CustomerTaxExempt)
	case index.charge != nil && strings.TrimSpace(index.charge.CustomerTaxExempt) != "":
		return strings.TrimSpace(index.charge.CustomerTaxExempt)
	case index.customer != nil:
		return strings.TrimSpace(index.customer.TaxExempt)
	default:
		return ""
	}
}

func resolveAutomaticTax(index snapshotIndex) (bool, *string) {
	switch {
	case index.invoice != nil:
		return index.invoice.AutomaticTax.Enabled, strPtr(strings.TrimSpace(index.invoice.AutomaticTax.Status))
	case index.checkoutSession != nil:
		return index.checkoutSession.AutomaticTax.Enabled, strPtr(strings.TrimSpace(index.checkoutSession.AutomaticTax.Status))
	default:
		return false, nil
	}
}

func resolveStripeTax(index snapshotIndex) (bool, *int64, bool, bool, []string) {
	reasonSet := map[string]struct{}{}
	var total int64
	known := false
	if index.invoice != nil {
		for _, line := range index.invoice.Lines.Data {
			for _, item := range line.Taxes {
				total += item.Amount
				known = true
				reason := strings.ToLower(strings.TrimSpace(item.TaxabilityReason))
				if reason != "" {
					reasonSet[reason] = struct{}{}
				}
			}
		}
		if !known && index.invoice.AutomaticTax.Enabled && strings.EqualFold(strings.TrimSpace(index.invoice.AutomaticTax.Status), "complete") {
			known = true
		}
	} else if index.checkoutSession != nil {
		total = index.checkoutSession.TotalDetails.AmountTax
		if index.checkoutSession.AutomaticTax.Enabled && strings.EqualFold(strings.TrimSpace(index.checkoutSession.AutomaticTax.Status), "complete") {
			known = true
		}
		if total > 0 || known {
			known = true
		}
	}
	reasons := make([]string, 0, len(reasonSet))
	for reason := range reasonSet {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	var amount *int64
	if known {
		amount = &total
	}
	return known, amount, hasReason(reasons, "reverse_charge"), hasReason(reasons, "zero_rated"), reasons
}

func hasReason(reasons []string, wanted string) bool {
	wanted = strings.ToLower(strings.TrimSpace(wanted))
	for _, reason := range reasons {
		if reason == wanted {
			return true
		}
	}
	return false
}

func resolveBuyerIsBusiness(vatNumber *string, vatVerified bool, reverseCharge bool, customerTaxExempt string, manual []domain.ManualTaxEvidence) bool {
	for _, ev := range manual {
		if ev.BuyerIsBusiness != nil {
			return *ev.BuyerIsBusiness
		}
	}
	if vatVerified {
		return true
	}
	if vatNumber != nil && strings.TrimSpace(*vatNumber) != "" {
		return true
	}
	return reverseCharge || strings.EqualFold(strings.TrimSpace(customerTaxExempt), "reverse")
}

func resolveReportability(country, saleType *string, business bool, vatNumber *string, vatVerified, taxKnown bool, manual []domain.ManualTaxEvidence) (string, string) {
	if country == nil || strings.TrimSpace(*country) == "" {
		return domain.NeedsManualEvidence, "missing customer country evidence"
	}
	if saleType == nil || strings.TrimSpace(*saleType) == "" {
		return domain.NeedsReview, "sale type could not be resolved"
	}
	if support.IsEUCountry(*country) && *country != "SE" && business {
		if vatNumber == nil || strings.TrimSpace(*vatNumber) == "" {
			return domain.NeedsManualEvidence, "EU B2B sale requires buyer VAT number"
		}
		if !vatVerified && !manualVATConfirmation(manual) {
			return domain.NeedsManualEvidence, "EU B2B sale requires verified or manually confirmed buyer VAT number"
		}
	}
	if support.IsEUCountry(*country) && *country != "SE" && !business && !taxKnown {
		return domain.NeedsManualEvidence, "EU B2C sale requires Stripe Tax evidence"
	}
	return domain.Reportable, ""
}

func manualVATConfirmation(items []domain.ManualTaxEvidence) bool {
	for _, item := range items {
		if item.BuyerVATVerified != nil && *item.BuyerVATVerified {
			return true
		}
	}
	return false
}

func classifyStatus(country *string, business bool) *string {
	if country == nil || strings.TrimSpace(*country) == "" {
		return nil
	}
	cc := support.NormalizeCountry(*country)
	switch {
	case cc == "SE" && business:
		return strPtr(domain.TaxStatusSEB2B)
	case cc == "SE":
		return strPtr(domain.TaxStatusSEB2C)
	case support.IsEUCountry(cc) && business:
		value := "EU_" + cc + "_B2B"
		return &value
	case support.IsEUCountry(cc):
		value := "EU_" + cc + "_B2C"
		return &value
	default:
		return strPtr(domain.TaxStatusOutsideEU)
	}
}

func resolveSourceCurrency(index snapshotIndex) *string {
	switch {
	case index.invoice != nil && strings.TrimSpace(index.invoice.Currency) != "":
		return strPtr(strings.ToUpper(strings.TrimSpace(index.invoice.Currency)))
	case index.checkoutSession != nil && strings.TrimSpace(index.checkoutSession.Currency) != "":
		return strPtr(strings.ToUpper(strings.TrimSpace(index.checkoutSession.Currency)))
	case index.paymentIntent != nil && strings.TrimSpace(index.paymentIntent.Currency) != "":
		return strPtr(strings.ToUpper(strings.TrimSpace(index.paymentIntent.Currency)))
	case index.charge != nil && strings.TrimSpace(index.charge.Currency) != "":
		return strPtr(strings.ToUpper(strings.TrimSpace(index.charge.Currency)))
	case index.refund != nil && strings.TrimSpace(index.refund.Currency) != "":
		return strPtr(strings.ToUpper(strings.TrimSpace(index.refund.Currency)))
	default:
		return nil
	}
}

func resolveSourceAmount(index snapshotIndex) *int64 {
	switch {
	case index.invoice != nil && index.invoice.Total != 0:
		return &index.invoice.Total
	case index.checkoutSession != nil && index.checkoutSession.AmountTotal != 0:
		return &index.checkoutSession.AmountTotal
	case index.paymentIntent != nil && index.paymentIntent.Amount != 0:
		return &index.paymentIntent.Amount
	case index.charge != nil && index.charge.Amount != 0:
		return &index.charge.Amount
	case index.refund != nil && index.refund.Amount != 0:
		return &index.refund.Amount
	default:
		return nil
	}
}

func resolveInvoicePDF(index snapshotIndex) *string {
	if index.invoice == nil || strings.TrimSpace(index.invoice.InvoicePDF) == "" {
		return nil
	}
	return strPtr(strings.TrimSpace(index.invoice.InvoicePDF))
}

func resolvePaymentMethodCountry(index snapshotIndex) string {
	if index.charge == nil || index.charge.PaymentMethodDetails == nil || index.charge.PaymentMethodDetails.Card == nil {
		return ""
	}
	return support.NormalizeCountry(index.charge.PaymentMethodDetails.Card.Country)
}

func reviewPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func strPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func int64OrNil(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}
