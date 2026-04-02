package store

import (
	"context"
	"fmt"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/google/uuid"
)

func (q *Queries) UpsertAccountingFacts(ctx context.Context, facts []domain.AccountingFact) error {
	sourceGroups := map[string]struct{}{}
	for _, fact := range facts {
		sourceGroups[fact.SourceGroupID] = struct{}{}
	}

	for sourceGroupID := range sourceGroups {
		rows, err := q.db.Query(ctx, `SELECT status FROM accounting_facts WHERE source_group_id = $1`, sourceGroupID)
		if err != nil {
			return fmt.Errorf("list existing accounting facts for %s: %w", sourceGroupID, err)
		}

		locked := false
		for rows.Next() {
			var status string
			if err := rows.Scan(&status); err != nil {
				rows.Close()
				return fmt.Errorf("scan existing accounting fact status: %w", err)
			}
			if status == domain.FactStatusBatched || status == domain.FactStatusPosted || status == domain.FactStatusReversed {
				locked = true
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("iterate existing accounting fact statuses: %w", err)
		}
		rows.Close()

		if locked {
			return fmt.Errorf("accounting facts for source group %s are already locked or posted; correction flow is required", sourceGroupID)
		}

		if _, err := q.db.Exec(ctx, `
			DELETE FROM accounting_facts
			WHERE source_group_id = $1 AND status IN ('pending', 'needs_review', 'failed')
		`, sourceGroupID); err != nil {
			return fmt.Errorf("delete replaceable accounting facts for %s: %w", sourceGroupID, err)
		}
	}

	for _, fact := range facts {
		_, err := q.db.Exec(ctx, `
			INSERT INTO accounting_facts (
				id, bokio_company_id, stripe_account_id, tax_case_id, source_group_id, source_object_type,
				source_object_id, stripe_balance_transaction_id, stripe_event_id, fact_type,
				posting_date, market_code, vat_treatment, source_currency, source_amount_minor,
				amount_sek_ore, bokio_account, direction, status, review_reason, payload
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
				$11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
			)
		`, fact.ID, fact.BokioCompanyID, fact.StripeAccountID, fact.TaxCaseID, fact.SourceGroupID, fact.SourceObjectType,
			fact.SourceObjectID, fact.StripeBalanceTransactionID, fact.StripeEventID, fact.FactType,
			fact.PostingDate, fact.MarketCode, fact.VATTreatment, fact.SourceCurrency, fact.SourceAmountMinor,
			fact.AmountSEKOre, fact.BokioAccount, fact.Direction, fact.Status, fact.ReviewReason, []byte(fact.Payload))
		if err != nil {
			return fmt.Errorf("upsert accounting fact %s: %w", fact.SourceGroupID, err)
		}
	}
	return nil
}

func (q *Queries) ListPendingAccountingFacts(ctx context.Context, companyID uuid.UUID, postingDate time.Time) ([]domain.AccountingFact, error) {
	rows, err := q.db.Query(ctx, `
		SELECT id, bokio_company_id, stripe_account_id, tax_case_id, source_group_id, source_object_type,
			source_object_id, stripe_balance_transaction_id, stripe_event_id, fact_type,
			posting_date, market_code, vat_treatment, source_currency, source_amount_minor,
			amount_sek_ore, bokio_account, direction, status, review_reason, payload,
			created_at, updated_at
		FROM accounting_facts
		WHERE bokio_company_id = $1 AND posting_date = $2 AND status IN ('pending', 'needs_review', 'batched')
		ORDER BY source_group_id, fact_type, bokio_account, direction
	`, companyID, postingDate)
	if err != nil {
		return nil, fmt.Errorf("list pending facts: %w", err)
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
		return nil, fmt.Errorf("iterate pending facts: %w", err)
	}
	return facts, nil
}

func (q *Queries) ListFactsByTaxCaseIDs(ctx context.Context, ids []uuid.UUID) ([]domain.AccountingFact, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := q.db.Query(ctx, `
		SELECT id, bokio_company_id, stripe_account_id, tax_case_id, source_group_id, source_object_type,
			source_object_id, stripe_balance_transaction_id, stripe_event_id, fact_type,
			posting_date, market_code, vat_treatment, source_currency, source_amount_minor,
			amount_sek_ore, bokio_account, direction, status, review_reason, payload,
			created_at, updated_at
		FROM accounting_facts
		WHERE tax_case_id = ANY($1)
		ORDER BY source_group_id, fact_type, bokio_account, direction
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("list facts by tax case ids: %w", err)
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
		return nil, fmt.Errorf("iterate facts by tax case ids: %w", err)
	}
	return facts, nil
}
