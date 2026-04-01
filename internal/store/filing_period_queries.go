package store

import (
	"context"
	"fmt"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/google/uuid"
)

func (q *Queries) UpsertFilingPeriods(ctx context.Context, periods []domain.FilingPeriod) error {
	for _, period := range periods {
		_, err := q.db.Exec(ctx, `
			INSERT INTO filing_periods (
				kind, period, bokio_company_id, deadline_date, first_send_at, last_evaluation_status
			) VALUES ($1, $2, $3, $4, $5, COALESCE(NULLIF($6, ''), 'pending'))
			ON CONFLICT (kind, period, bokio_company_id) DO UPDATE SET
				deadline_date = EXCLUDED.deadline_date,
				first_send_at = EXCLUDED.first_send_at,
				updated_at = now()
		`, period.Kind, period.Period, period.BokioCompanyID, period.DeadlineDate, period.FirstSendAt, period.LastEvaluationStatus)
		if err != nil {
			return fmt.Errorf("upsert filing period %s/%s: %w", period.Kind, period.Period, err)
		}
	}
	return nil
}

func (q *Queries) GetFilingPeriod(ctx context.Context, companyID uuid.UUID, kind, period string) (domain.FilingPeriod, error) {
	row := q.db.QueryRow(ctx, `
		SELECT kind, period, bokio_company_id, deadline_date, first_send_at, last_evaluated_at,
			last_evaluation_status, zero_reminder_sent_at, submitted_at, created_at, updated_at
		FROM filing_periods
		WHERE bokio_company_id = $1 AND kind = $2 AND period = $3
	`, companyID, kind, period)

	return scanFilingPeriod(row)
}

func (q *Queries) ListDueFilingPeriods(ctx context.Context, companyID uuid.UUID, asOf time.Time) ([]domain.FilingPeriod, error) {
	rows, err := q.db.Query(ctx, `
		SELECT kind, period, bokio_company_id, deadline_date, first_send_at, last_evaluated_at,
			last_evaluation_status, zero_reminder_sent_at, submitted_at, created_at, updated_at
		FROM filing_periods
		WHERE bokio_company_id = $1
			AND first_send_at <= $2
			AND submitted_at IS NULL
		ORDER BY deadline_date, kind, period
	`, companyID, asOf)
	if err != nil {
		return nil, fmt.Errorf("list due filing periods: %w", err)
	}
	defer rows.Close()

	var periods []domain.FilingPeriod
	for rows.Next() {
		period, err := scanFilingPeriod(rows)
		if err != nil {
			return nil, err
		}
		periods = append(periods, period)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due filing periods: %w", err)
	}
	return periods, nil
}

func (q *Queries) UpdateFilingPeriodEvaluation(ctx context.Context, companyID uuid.UUID, kind, period, status string, evaluatedAt time.Time, zeroReminderSentAt *time.Time) error {
	_, err := q.db.Exec(ctx, `
		UPDATE filing_periods
		SET last_evaluated_at = $4,
			last_evaluation_status = $5,
			zero_reminder_sent_at = COALESCE($6, zero_reminder_sent_at),
			updated_at = now()
		WHERE bokio_company_id = $1 AND kind = $2 AND period = $3
	`, companyID, kind, period, evaluatedAt, status, zeroReminderSentAt)
	if err != nil {
		return fmt.Errorf("update filing period evaluation: %w", err)
	}
	return nil
}

func (q *Queries) MarkFilingPeriodSubmitted(ctx context.Context, companyID uuid.UUID, kind, period string, submittedAt time.Time) error {
	_, err := q.db.Exec(ctx, `
		UPDATE filing_periods
		SET submitted_at = $4,
			last_evaluation_status = 'submitted',
			updated_at = now()
		WHERE bokio_company_id = $1 AND kind = $2 AND period = $3
	`, companyID, kind, period, submittedAt)
	if err != nil {
		return fmt.Errorf("mark filing period submitted: %w", err)
	}
	return nil
}

func (q *Queries) ListEntryPeriods(ctx context.Context, companyID uuid.UUID, kind string) ([]string, error) {
	var sqlText string
	switch kind {
	case domain.FilingKindOSSUnion:
		sqlText = `SELECT DISTINCT filing_period FROM oss_union_entries WHERE bokio_company_id = $1`
	case domain.FilingKindPeriodicSummary:
		sqlText = `SELECT DISTINCT filing_period FROM periodic_summary_entries WHERE bokio_company_id = $1`
	default:
		return nil, fmt.Errorf("unsupported filing kind %q", kind)
	}

	rows, err := q.db.Query(ctx, sqlText, companyID)
	if err != nil {
		return nil, fmt.Errorf("list entry periods: %w", err)
	}
	defer rows.Close()

	var periods []string
	for rows.Next() {
		var period string
		if err := rows.Scan(&period); err != nil {
			return nil, fmt.Errorf("scan entry period: %w", err)
		}
		periods = append(periods, period)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entry periods: %w", err)
	}
	return periods, nil
}
