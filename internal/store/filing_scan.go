package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func scanOSSUnionEntry(row interface{ Scan(dest ...any) error }) (domain.OSSUnionEntry, error) {
	var entry domain.OSSUnionEntry
	var stripeEventID, correctionTargetPeriod, reviewReason sql.NullString
	var payload []byte
	if err := row.Scan(
		&entry.ID,
		&entry.BokioCompanyID,
		&entry.SourceGroupID,
		&entry.SourceObjectType,
		&entry.SourceObjectID,
		&stripeEventID,
		&entry.OriginalSupplyPeriod,
		&entry.FilingPeriod,
		&correctionTargetPeriod,
		&entry.ConsumptionCountry,
		&entry.OriginCountry,
		&entry.OriginIdentifier,
		&entry.SaleType,
		&entry.VATRateBasisPoints,
		&entry.TaxableAmountEURCents,
		&entry.VATAmountEURCents,
		&entry.ReviewState,
		&reviewReason,
		&payload,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	); err != nil {
		return domain.OSSUnionEntry{}, fmt.Errorf("scan oss union entry: %w", err)
	}
	if stripeEventID.Valid {
		entry.StripeEventID = &stripeEventID.String
	}
	if correctionTargetPeriod.Valid {
		entry.CorrectionTargetPeriod = &correctionTargetPeriod.String
	}
	if reviewReason.Valid {
		entry.ReviewReason = &reviewReason.String
	}
	entry.Payload = payload
	return entry, nil
}

func scanPeriodicSummaryEntry(row interface{ Scan(dest ...any) error }) (domain.PeriodicSummaryEntry, error) {
	var entry domain.PeriodicSummaryEntry
	var stripeEventID, reviewReason sql.NullString
	var payload []byte
	if err := row.Scan(
		&entry.ID,
		&entry.BokioCompanyID,
		&entry.SourceGroupID,
		&entry.SourceObjectType,
		&entry.SourceObjectID,
		&stripeEventID,
		&entry.FilingPeriod,
		&entry.BuyerVATNumber,
		&entry.RowType,
		&entry.AmountSEKOre,
		&entry.ExportedAmountSEK,
		&entry.ReviewState,
		&reviewReason,
		&payload,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	); err != nil {
		return domain.PeriodicSummaryEntry{}, fmt.Errorf("scan periodic summary entry: %w", err)
	}
	if stripeEventID.Valid {
		entry.StripeEventID = &stripeEventID.String
	}
	if reviewReason.Valid {
		entry.ReviewReason = &reviewReason.String
	}
	entry.Payload = payload
	return entry, nil
}

func scanFilingPeriod(row interface{ Scan(dest ...any) error }) (domain.FilingPeriod, error) {
	var period domain.FilingPeriod
	var lastEvaluatedAt, zeroReminderSentAt, submittedAt sql.NullTime
	if err := row.Scan(
		&period.Kind,
		&period.Period,
		&period.BokioCompanyID,
		&period.DeadlineDate,
		&period.FirstSendAt,
		&lastEvaluatedAt,
		&period.LastEvaluationStatus,
		&zeroReminderSentAt,
		&submittedAt,
		&period.CreatedAt,
		&period.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.FilingPeriod{}, ErrNotFound
		}
		return domain.FilingPeriod{}, fmt.Errorf("scan filing period: %w", err)
	}
	if lastEvaluatedAt.Valid {
		period.LastEvaluatedAt = &lastEvaluatedAt.Time
	}
	if zeroReminderSentAt.Valid {
		period.ZeroReminderSentAt = &zeroReminderSentAt.Time
	}
	if submittedAt.Valid {
		period.SubmittedAt = &submittedAt.Time
	}
	return period, nil
}

func scanFilingExport(row interface{ Scan(dest ...any) error }) (domain.FilingExport, error) {
	var export domain.FilingExport
	var filename sql.NullString
	var emailedAt sql.NullTime
	var supersededBy uuid.NullUUID
	var summary []byte
	if err := row.Scan(
		&export.ID,
		&export.Kind,
		&export.Period,
		&export.BokioCompanyID,
		&export.Version,
		&export.Checksum,
		&filename,
		&export.Content,
		&summary,
		&emailedAt,
		&supersededBy,
		&export.CreatedAt,
		&export.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.FilingExport{}, ErrNotFound
		}
		return domain.FilingExport{}, fmt.Errorf("scan filing export: %w", err)
	}
	if filename.Valid {
		export.Filename = &filename.String
	}
	if emailedAt.Valid {
		export.EmailedAt = &emailedAt.Time
	}
	if supersededBy.Valid {
		export.SupersededBy = &supersededBy.UUID
	}
	export.Summary = json.RawMessage(summary)
	return export, nil
}
