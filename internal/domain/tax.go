package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	TaxStatusSEB2C      = "SE_B2C"
	TaxStatusSEB2B      = "SE_B2B"
	TaxStatusOutsideEU  = "OUTSIDE_EU"
	Reportable          = "reportable"
	NeedsManualEvidence = "needs_manual_evidence"
	NeedsReview         = "needs_review"
)

type TaxCase struct {
	ID                     uuid.UUID
	BokioCompanyID         uuid.UUID
	RootObjectType         string
	RootObjectID           string
	Livemode               bool
	SourceCurrency         *string
	SourceAmountMinor      *int64
	SaleType               *string
	Country                *string
	CountrySource          *string
	BuyerVATNumber         *string
	BuyerVATVerified       bool
	BuyerIsBusiness        bool
	TaxStatus              *string
	ReportabilityState     string
	ReviewReason           *string
	AutomaticTaxEnabled    bool
	AutomaticTaxStatus     *string
	StripeTaxAmountKnown   bool
	StripeTaxAmountMinor   *int64
	StripeTaxReverseCharge bool
	StripeTaxZeroRated     bool
	InvoicePDFURL          *string
	Dossier                json.RawMessage
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type TaxCaseObject struct {
	TaxCaseID  uuid.UUID
	ObjectType string
	ObjectID   string
	ObjectRole string
	CreatedAt  time.Time
}

type ManualTaxEvidence struct {
	ID               uuid.UUID       `json:"id"`
	TaxCaseID        uuid.UUID       `json:"tax_case_id"`
	Country          *string         `json:"country,omitempty"`
	CountrySource    *string         `json:"country_source,omitempty"`
	BuyerVATNumber   *string         `json:"buyer_vat_number,omitempty"`
	BuyerVATVerified *bool           `json:"buyer_vat_verified,omitempty"`
	BuyerIsBusiness  *bool           `json:"buyer_is_business,omitempty"`
	SaleType         *string         `json:"sale_type,omitempty"`
	Note             *string         `json:"note,omitempty"`
	Payload          json.RawMessage `json:"payload,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}
