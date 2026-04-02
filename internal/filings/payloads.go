package filings

import (
	"encoding/json"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/google/uuid"
)

type stripeEventEnvelope struct {
	Data struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

type chargePayload struct {
	ID                string            `json:"id"`
	Amount            int64             `json:"amount"`
	Currency          string            `json:"currency"`
	Created           int64             `json:"created"`
	Customer          string            `json:"customer"`
	Invoice           string            `json:"invoice"`
	CustomerTaxExempt string            `json:"customer_tax_exempt"`
	Metadata          map[string]string `json:"metadata"`
	BillingDetails    billingDetails    `json:"billing_details"`
	CustomerDetails   *customerDetails  `json:"customer_details"`
	Refunds           struct {
		Data []refundPayload `json:"data"`
	} `json:"refunds"`
}

type refundPayload struct {
	ID       string            `json:"id"`
	Amount   int64             `json:"amount"`
	Currency string            `json:"currency"`
	Created  int64             `json:"created"`
	Metadata map[string]string `json:"metadata"`
}

type invoicePayload struct {
	ID                string                 `json:"id"`
	Customer          string                 `json:"customer"`
	CustomerTaxExempt string                 `json:"customer_tax_exempt"`
	CustomerAddress   *address               `json:"customer_address"`
	CustomerShipping  *shipping              `json:"customer_shipping"`
	CustomerTaxIDs    []invoiceCustomerTaxID `json:"customer_tax_ids"`
	Metadata          map[string]string      `json:"metadata"`
}

type invoiceCustomerTaxID struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type customerPayload struct {
	ID        string            `json:"id"`
	Address   *address          `json:"address"`
	Shipping  *shipping         `json:"shipping"`
	TaxExempt string            `json:"tax_exempt"`
	Metadata  map[string]string `json:"metadata"`
}

type billingDetails struct {
	Address address `json:"address"`
}

type customerDetails struct {
	Address address `json:"address"`
}

type shipping struct {
	Address address `json:"address"`
}

type address struct {
	Country string `json:"country"`
}

type filingFacts struct {
	Representative domain.AccountingFact
	MarketCode     string
	VATTreatment   string
	GrossSEKOre    int64
	RevenueSEKOre  int64
	VATSEKOre      int64
	SourceCurrency string
	SourceAmount   int64
	HasSourceMinor bool
	ReviewReason   string
}

type filingContext struct {
	TaxCaseID          *uuid.UUID
	TaxStatus          string
	ReportabilityState string
	GroupID            string
	SourceObjectType   string
	SourceObjectID     string
	StripeEventID      *string
	PostingDate        time.Time
	OriginalSupplyDate time.Time
	MarketCode         string
	VATTreatment       string
	SaleCategory       string
	BuyerVATNumber     string
	Country            string
	ReviewReason       string
	AmountSEKOre       int64
	RevenueSEKOre      int64
	VATSEKOre          int64
	SourceCurrency     string
	SourceAmountMinor  int64
	HasSourceAmount    bool
	SaleType           string
	ShippingEvidence   bool
	Charge             *chargePayload
	Refund             *refundPayload
	Invoice            *invoicePayload
	Customer           *customerPayload
	AllMetadata        map[string]string
}
