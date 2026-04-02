package accounting

import (
	"time"

	"github.com/google/uuid"
)

type SaleInput struct {
	BokioCompanyID             uuid.UUID
	StripeAccountID            string
	TaxCaseID                  *uuid.UUID
	SourceObjectType           string
	SourceObjectID             string
	SourceGroupPrefix          string
	StripeBalanceTransactionID string
	StripeEventID              string
	ChargeDate                 time.Time
	AvailableOn                *time.Time
	SourceCurrency             string
	GrossMinor                 int64
	FeeCurrency                string
	FeeMinor                   int64
	GrossSEKOre                int64
	FeeSEKOre                  int64
	MarketCode                 string
	VATTreatment               string
	RevenueAccount             int
	OutputVATAccount           int
	ReceivableAccount          int
	StripeBalanceAccount       int
	FeeExpenseAccount          int
	FeeInputVATAccount         int
	FeeOutputVATAccount        int
	RevenueSEKOre              int64
	VATSEKOre                  int64
	FeeReverseVATSEKOre        int64
	Payload                    any
}

type RefundInput struct {
	BokioCompanyID             uuid.UUID
	StripeAccountID            string
	TaxCaseID                  *uuid.UUID
	SourceObjectID             string
	SourceGroupID              string
	StripeBalanceTransactionID string
	StripeEventID              string
	PostingDate                time.Time
	SourceCurrency             string
	GrossMinor                 int64
	GrossSEKOre                int64
	MarketCode                 string
	VATTreatment               string
	RevenueAccount             int
	OutputVATAccount           int
	ReceivableAccount          int
	RevenueSEKOre              int64
	VATSEKOre                  int64
	Payload                    any
}

type PayoutInput struct {
	BokioCompanyID             uuid.UUID
	StripeAccountID            string
	TaxCaseID                  *uuid.UUID
	SourceObjectID             string
	SourceGroupID              string
	StripeBalanceTransactionID string
	StripeEventID              string
	PostingDate                time.Time
	SourceCurrency             string
	AmountMinor                int64
	AmountSEKOre               int64
	BankAccount                int
	StripeBalanceAccount       int
	Payload                    any
}

type DisputeInput struct {
	BokioCompanyID             uuid.UUID
	StripeAccountID            string
	TaxCaseID                  *uuid.UUID
	SourceObjectID             string
	SourceGroupID              string
	StripeBalanceTransactionID string
	StripeEventID              string
	PostingDate                time.Time
	SourceCurrency             string
	AmountMinor                int64
	AmountSEKOre               int64
	DisputeAccount             int
	StripeBalanceAccount       int
	Payload                    any
}

type ReviewTransferInput struct {
	BokioCompanyID             uuid.UUID
	StripeAccountID            string
	TaxCaseID                  *uuid.UUID
	SourceGroupID              string
	SourceObjectType           string
	SourceObjectID             string
	StripeBalanceTransactionID string
	StripeEventID              string
	PostingDate                time.Time
	MarketCode                 string
	VATTreatment               string
	SourceCurrency             string
	SourceAmountMinor          int64
	AmountSEKOre               int64
	DebitFactType              string
	CreditFactType             string
	DebitAccount               int
	CreditAccount              int
	ReviewReason               string
	Payload                    any
}

type SaleClassificationInput struct {
	Country           string
	IsB2B             bool
	GrossSEKOre       int64
	ExplicitVATSEKOre *int64
	Evidence          SaleEvidence
}

type SaleEvidence struct {
	CountryEvidence        bool
	CountrySource          string
	VATMode                string
	SaleCategory           string
	OSSApplied             bool
	ExportEvidence         bool
	CustomerVATID          string
	CustomerVATValidated   bool
	CustomerTaxExempt      string
	AutomaticTaxEnabled    bool
	AutomaticTaxStatus     string
	StripeTaxAmountKnown   bool
	StripeTaxReverseCharge bool
	StripeTaxZeroRated     bool
	TaxabilityReasons      []string
	AllowCountryFallback   bool
}

type SalesResolution struct {
	MarketCode       string
	VATTreatment     string
	RevenueAccount   int
	OutputVATAccount int
	RevenueSEKOre    int64
	VATSEKOre        int64
	ReviewReason     string
}
