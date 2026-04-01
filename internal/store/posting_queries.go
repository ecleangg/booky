package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (q *Queries) CreatePostingRun(ctx context.Context, run domain.PostingRun) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO posting_runs (
			id, bokio_company_id, posting_date, timezone, run_type, sequence_no, status,
			config_snapshot, summary
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, run.ID, run.BokioCompanyID, run.PostingDate, run.Timezone, run.RunType, run.SequenceNo, run.Status, []byte(run.ConfigSnapshot), []byte(run.Summary))
	if err != nil {
		return fmt.Errorf("create posting run: %w", err)
	}
	return nil
}

func (q *Queries) UpdatePostingRun(ctx context.Context, runID uuid.UUID, status string, summary json.RawMessage, errMsg *string) error {
	_, err := q.db.Exec(ctx, `
		UPDATE posting_runs
		SET status = $2, summary = COALESCE($3, summary), error_message = $4,
			started_at = CASE WHEN $2 = 'started' THEN now() ELSE started_at END,
			finished_at = CASE WHEN $2 IN ('completed', 'failed') THEN now() WHEN $2 = 'started' THEN NULL ELSE finished_at END
		WHERE id = $1
	`, runID, status, nullableJSON(summary), errMsg)
	if err != nil {
		return fmt.Errorf("update posting run: %w", err)
	}
	return nil
}

func (q *Queries) ResetPostingRun(ctx context.Context, runID uuid.UUID, configSnapshot, summary json.RawMessage) error {
	_, err := q.db.Exec(ctx, `
		UPDATE posting_runs
		SET status = 'started',
			config_snapshot = $2,
			summary = COALESCE($3, '{}'::jsonb),
			error_message = NULL,
			started_at = now(),
			finished_at = NULL
		WHERE id = $1
	`, runID, nullableJSON(configSnapshot), nullableJSON(summary))
	if err != nil {
		return fmt.Errorf("reset posting run: %w", err)
	}
	return nil
}

func (q *Queries) GetPostingRunByDate(ctx context.Context, companyID uuid.UUID, postingDate time.Time, runType string) (domain.PostingRun, error) {
	row := q.db.QueryRow(ctx, `
		SELECT id, bokio_company_id, posting_date, timezone, run_type, sequence_no, status,
			config_snapshot, summary, started_at, finished_at, error_message
		FROM posting_runs
		WHERE bokio_company_id = $1 AND posting_date = $2 AND run_type = $3
		ORDER BY sequence_no DESC
		LIMIT 1
	`, companyID, postingDate, runType)

	var run domain.PostingRun
	var finishedAt sql.NullTime
	var errorMessage sql.NullString
	var configSnapshot, summary []byte
	if err := row.Scan(&run.ID, &run.BokioCompanyID, &run.PostingDate, &run.Timezone, &run.RunType, &run.SequenceNo, &run.Status,
		&configSnapshot, &summary, &run.StartedAt, &finishedAt, &errorMessage); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PostingRun{}, ErrNotFound
		}
		return domain.PostingRun{}, fmt.Errorf("get posting run: %w", err)
	}
	run.ConfigSnapshot = configSnapshot
	run.Summary = summary
	if finishedAt.Valid {
		run.FinishedAt = &finishedAt.Time
	}
	if errorMessage.Valid {
		run.ErrorMessage = &errorMessage.String
	}
	return run, nil
}

func (q *Queries) AttachFactsToRun(ctx context.Context, runID uuid.UUID, factIDs []uuid.UUID) error {
	for _, id := range factIDs {
		_, err := q.db.Exec(ctx, `
			INSERT INTO posting_run_facts (posting_run_id, accounting_fact_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, runID, id)
		if err != nil {
			return fmt.Errorf("attach fact to run: %w", err)
		}
	}
	return nil
}

func (q *Queries) MarkFactsStatus(ctx context.Context, factIDs []uuid.UUID, status string) error {
	for _, id := range factIDs {
		_, err := q.db.Exec(ctx, `UPDATE accounting_facts SET status = $2, updated_at = now() WHERE id = $1`, id, status)
		if err != nil {
			return fmt.Errorf("mark fact status: %w", err)
		}
	}
	return nil
}

func (q *Queries) UpsertBokioJournal(ctx context.Context, journal domain.BokioJournal) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO bokio_journals (
			posting_run_id, bokio_company_id, bokio_journal_entry_id, bokio_journal_entry_number,
			bokio_upload_id, bokio_journal_title, posting_date, attachment_checksum,
			reversed_at, reversed_by_journal_entry_id
		) VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8, $9, $10)
		ON CONFLICT (posting_run_id) DO UPDATE SET
			bokio_journal_entry_id = EXCLUDED.bokio_journal_entry_id,
			bokio_journal_entry_number = EXCLUDED.bokio_journal_entry_number,
			bokio_upload_id = EXCLUDED.bokio_upload_id,
			bokio_journal_title = EXCLUDED.bokio_journal_title,
			posting_date = EXCLUDED.posting_date,
			attachment_checksum = EXCLUDED.attachment_checksum,
			reversed_at = EXCLUDED.reversed_at,
			reversed_by_journal_entry_id = EXCLUDED.reversed_by_journal_entry_id
	`, journal.PostingRunID, journal.BokioCompanyID, journal.BokioJournalEntryID, journal.BokioJournalEntryNo,
		journal.BokioUploadID, journal.BokioJournalTitle, journal.PostingDate, journal.AttachmentChecksum,
		journal.ReversedAt, journal.ReversedByJournalID)
	if err != nil {
		return fmt.Errorf("upsert bokio journal: %w", err)
	}
	return nil
}

func (q *Queries) GetBokioJournalByRunID(ctx context.Context, runID uuid.UUID) (domain.BokioJournal, error) {
	row := q.db.QueryRow(ctx, `
		SELECT posting_run_id, bokio_company_id, bokio_journal_entry_id, bokio_journal_entry_number,
			bokio_upload_id, bokio_journal_title, posting_date, attachment_checksum,
			created_at, reversed_at, reversed_by_journal_entry_id
		FROM bokio_journals
		WHERE posting_run_id = $1
	`, runID)

	var journal domain.BokioJournal
	var uploadID, reversedBy uuid.NullUUID
	var reversedAt sql.NullTime
	if err := row.Scan(&journal.PostingRunID, &journal.BokioCompanyID, &journal.BokioJournalEntryID, &journal.BokioJournalEntryNo,
		&uploadID, &journal.BokioJournalTitle, &journal.PostingDate, &journal.AttachmentChecksum,
		&journal.CreatedAt, &reversedAt, &reversedBy); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.BokioJournal{}, ErrNotFound
		}
		return domain.BokioJournal{}, fmt.Errorf("get bokio journal: %w", err)
	}
	if uploadID.Valid {
		journal.BokioUploadID = &uploadID.UUID
	}
	if reversedAt.Valid {
		journal.ReversedAt = &reversedAt.Time
	}
	if reversedBy.Valid {
		journal.ReversedByJournalID = &reversedBy.UUID
	}
	return journal, nil
}

func (q *Queries) ListFactsByRun(ctx context.Context, runID uuid.UUID) ([]domain.AccountingFact, error) {
	rows, err := q.db.Query(ctx, `
		SELECT f.id, f.bokio_company_id, f.stripe_account_id, f.source_group_id, f.source_object_type,
			f.source_object_id, f.stripe_balance_transaction_id, f.stripe_event_id, f.fact_type,
			f.posting_date, f.market_code, f.vat_treatment, f.source_currency, f.source_amount_minor,
			f.amount_sek_ore, f.bokio_account, f.direction, f.status, f.review_reason, f.payload,
			f.created_at, f.updated_at
		FROM accounting_facts f
		JOIN posting_run_facts prf ON prf.accounting_fact_id = f.id
		WHERE prf.posting_run_id = $1
		ORDER BY f.source_group_id, f.fact_type, f.bokio_account, f.direction
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("list facts by run: %w", err)
	}
	defer rows.Close()

	var facts []domain.AccountingFact
	for rows.Next() {
		fact, err := scanAccountingFact(rows)
		if err != nil {
			return nil, err
		}
		facts = append(facts, fact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate facts by run: %w", err)
	}
	return facts, nil
}
