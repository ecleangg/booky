package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (q *Queries) ListFilingRelevantFactsByDateRange(ctx context.Context, companyID uuid.UUID, fromDate, toDate time.Time) ([]domain.AccountingFact, error) {
	rows, err := q.db.Query(ctx, `
		SELECT id, bokio_company_id, stripe_account_id, tax_case_id, source_group_id, source_object_type,
			source_object_id, stripe_balance_transaction_id, stripe_event_id, fact_type,
			posting_date, market_code, vat_treatment, source_currency, source_amount_minor,
			amount_sek_ore, bokio_account, direction, status, review_reason, payload,
			created_at, updated_at
		FROM accounting_facts
		WHERE bokio_company_id = $1
			AND posting_date >= $2
			AND posting_date <= $3
			AND (
				source_group_id LIKE 'charge:%:sale'
				OR source_group_id LIKE 'refund:%'
			)
		ORDER BY source_group_id, fact_type, created_at
	`, companyID, fromDate, toDate)
	if err != nil {
		return nil, fmt.Errorf("list filing relevant facts: %w", err)
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
		return nil, fmt.Errorf("iterate filing relevant facts: %w", err)
	}
	return facts, nil
}

func (q *Queries) GetObjectSnapshot(ctx context.Context, objectType, objectID string) (domain.ObjectSnapshot, error) {
	row := q.db.QueryRow(ctx, `
		SELECT object_type, stripe_object_id, livemode, payload, first_seen_at, last_synced_at
		FROM stripe_object_snapshots
		WHERE object_type = $1 AND stripe_object_id = $2
	`, objectType, objectID)

	var snapshot domain.ObjectSnapshot
	var payload []byte
	if err := row.Scan(&snapshot.ObjectType, &snapshot.ObjectID, &snapshot.Livemode, &payload, &snapshot.FirstSeenAt, &snapshot.LastSyncedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ObjectSnapshot{}, ErrNotFound
		}
		return domain.ObjectSnapshot{}, fmt.Errorf("get object snapshot: %w", err)
	}
	snapshot.Payload = payload
	return snapshot, nil
}
