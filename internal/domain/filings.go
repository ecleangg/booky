package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	FilingKindOSSUnion        = "oss_union"
	FilingKindPeriodicSummary = "periodic_summary"

	FilingReviewStateReady  = "ready"
	FilingReviewStateReview = "review"

	FilingPeriodStatusPending          = "pending"
	FilingPeriodStatusNoDataReminder   = "no_data_reminder_sent"
	FilingPeriodStatusExported         = "exported"
	FilingPeriodStatusUnchanged        = "unchanged"
	FilingPeriodStatusSubmitted        = "submitted"
	FilingPeriodStatusEvaluationFailed = "evaluation_failed"
)

type OSSUnionEntry struct {
	ID                     uuid.UUID
	BokioCompanyID         uuid.UUID
	SourceGroupID          string
	SourceObjectType       string
	SourceObjectID         string
	StripeEventID          *string
	OriginalSupplyPeriod   string
	FilingPeriod           string
	CorrectionTargetPeriod *string
	ConsumptionCountry     string
	OriginCountry          string
	OriginIdentifier       string
	SaleType               string
	VATRateBasisPoints     int
	TaxableAmountEURCents  int64
	VATAmountEURCents      int64
	ReviewState            string
	ReviewReason           *string
	Payload                json.RawMessage
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type PeriodicSummaryEntry struct {
	ID                uuid.UUID
	BokioCompanyID    uuid.UUID
	SourceGroupID     string
	SourceObjectType  string
	SourceObjectID    string
	StripeEventID     *string
	FilingPeriod      string
	BuyerVATNumber    string
	RowType           string
	AmountSEKOre      int64
	ExportedAmountSEK int64
	ReviewState       string
	ReviewReason      *string
	Payload           json.RawMessage
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type FilingPeriod struct {
	Kind                 string
	Period               string
	BokioCompanyID       uuid.UUID
	DeadlineDate         time.Time
	FirstSendAt          time.Time
	LastEvaluatedAt      *time.Time
	LastEvaluationStatus string
	ZeroReminderSentAt   *time.Time
	SubmittedAt          *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type FilingExport struct {
	ID             uuid.UUID
	Kind           string
	Period         string
	BokioCompanyID uuid.UUID
	Version        int
	Checksum       string
	Filename       *string
	Content        []byte
	Summary        json.RawMessage
	EmailedAt      *time.Time
	SupersededBy   *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type FXRate struct {
	Provider      string
	BaseCurrency  string
	QuoteCurrency string
	Period        string
	Rate          float64
	ObservedAt    time.Time
}
