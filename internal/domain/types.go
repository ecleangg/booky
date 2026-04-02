package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	FactStatusPending     = "pending"
	FactStatusBatched     = "batched"
	FactStatusPosted      = "posted"
	FactStatusReversed    = "reversed"
	FactStatusFailed      = "failed"
	FactStatusNeedsReview = "needs_review"

	DirectionDebit  = "debit"
	DirectionCredit = "credit"

	PostingRunTypeDailyClose = "daily_close"
	PostingRunTypeRepost     = "repost"

	PostingRunStatusStarted        = "started"
	PostingRunStatusJournalCreated = "journal_created"
	PostingRunStatusUploadCreated  = "upload_created"
	PostingRunStatusCompleted      = "completed"
	PostingRunStatusFailed         = "failed"
)

type StripeWebhookEvent struct {
	ID              string
	EventType       string
	Livemode        bool
	APIVersion      string
	StripeCreatedAt time.Time
	ReceivedAt      time.Time
	Payload         json.RawMessage
	ProcessedAt     *time.Time
	ProcessingError *string
}

type ObjectSnapshot struct {
	ObjectType   string
	ObjectID     string
	Livemode     bool
	Payload      json.RawMessage
	FirstSeenAt  time.Time
	LastSyncedAt time.Time
}

type BalanceTransaction struct {
	ID                string
	StripeAccountID   string
	SourceObjectType  string
	SourceObjectID    string
	Type              string
	ReportingCategory string
	Status            string
	Currency          string
	CurrencyExponent  int16
	AmountMinor       int64
	FeeMinor          int64
	NetMinor          int64
	ExchangeRate      *float64
	AmountSEKOre      *int64
	FeeSEKOre         *int64
	NetSEKOre         *int64
	OccurredAt        time.Time
	AvailableOn       *time.Time
	PayoutID          string
	SourceEventID     string
	Payload           json.RawMessage
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type AccountingFact struct {
	ID                         uuid.UUID
	BokioCompanyID             uuid.UUID
	StripeAccountID            string
	TaxCaseID                  *uuid.UUID
	SourceGroupID              string
	SourceObjectType           string
	SourceObjectID             string
	StripeBalanceTransactionID *string
	StripeEventID              *string
	FactType                   string
	PostingDate                time.Time
	MarketCode                 *string
	VATTreatment               *string
	SourceCurrency             *string
	SourceAmountMinor          *int64
	AmountSEKOre               int64
	BokioAccount               int
	Direction                  string
	Status                     string
	ReviewReason               *string
	Payload                    json.RawMessage
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

type PostingRun struct {
	ID             uuid.UUID
	BokioCompanyID uuid.UUID
	PostingDate    time.Time
	Timezone       string
	RunType        string
	SequenceNo     int
	Status         string
	ConfigSnapshot json.RawMessage
	Summary        json.RawMessage
	StartedAt      time.Time
	FinishedAt     *time.Time
	ErrorMessage   *string
}

type BokioJournal struct {
	PostingRunID        uuid.UUID
	BokioCompanyID      uuid.UUID
	BokioJournalEntryID uuid.UUID
	BokioJournalEntryNo string
	BokioUploadID       *uuid.UUID
	BokioJournalTitle   string
	PostingDate         time.Time
	AttachmentChecksum  string
	CreatedAt           time.Time
	ReversedAt          *time.Time
	ReversedByJournalID *uuid.UUID
}

type JournalItem struct {
	Account int     `json:"account"`
	Debit   float64 `json:"debit"`
	Credit  float64 `json:"credit"`
}

type JournalDraft struct {
	Title       string
	Date        time.Time
	GeneratedAt time.Time
	Items       []JournalItem
	Facts       []AccountingFact
	TaxCases    []TaxCase
	Summary     map[string]any
	PostingRun  PostingRun
	Description string
}
