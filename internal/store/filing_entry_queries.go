package store

import (
	"context"
	"fmt"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/google/uuid"
)

func (q *Queries) UpsertOSSUnionEntries(ctx context.Context, entries []domain.OSSUnionEntry) error {
	for _, entry := range entries {
		_, err := q.db.Exec(ctx, `
			INSERT INTO oss_union_entries (
				id, bokio_company_id, source_group_id, source_object_type, source_object_id, stripe_event_id,
				original_supply_period, filing_period, correction_target_period, consumption_country,
				origin_country, origin_identifier, sale_type, vat_rate_basis_points,
				taxable_amount_eur_cents, vat_amount_eur_cents, review_state, review_reason, payload
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
				$11, $12, $13, $14, $15, $16, $17, $18, $19
			)
			ON CONFLICT (source_group_id) DO UPDATE SET
				bokio_company_id = EXCLUDED.bokio_company_id,
				source_object_type = EXCLUDED.source_object_type,
				source_object_id = EXCLUDED.source_object_id,
				stripe_event_id = EXCLUDED.stripe_event_id,
				original_supply_period = EXCLUDED.original_supply_period,
				filing_period = EXCLUDED.filing_period,
				correction_target_period = EXCLUDED.correction_target_period,
				consumption_country = EXCLUDED.consumption_country,
				origin_country = EXCLUDED.origin_country,
				origin_identifier = EXCLUDED.origin_identifier,
				sale_type = EXCLUDED.sale_type,
				vat_rate_basis_points = EXCLUDED.vat_rate_basis_points,
				taxable_amount_eur_cents = EXCLUDED.taxable_amount_eur_cents,
				vat_amount_eur_cents = EXCLUDED.vat_amount_eur_cents,
				review_state = EXCLUDED.review_state,
				review_reason = EXCLUDED.review_reason,
				payload = EXCLUDED.payload,
				updated_at = now()
		`,
			entry.ID,
			entry.BokioCompanyID,
			entry.SourceGroupID,
			entry.SourceObjectType,
			entry.SourceObjectID,
			entry.StripeEventID,
			entry.OriginalSupplyPeriod,
			entry.FilingPeriod,
			entry.CorrectionTargetPeriod,
			entry.ConsumptionCountry,
			entry.OriginCountry,
			entry.OriginIdentifier,
			entry.SaleType,
			entry.VATRateBasisPoints,
			entry.TaxableAmountEURCents,
			entry.VATAmountEURCents,
			entry.ReviewState,
			entry.ReviewReason,
			[]byte(entry.Payload),
		)
		if err != nil {
			return fmt.Errorf("upsert oss union entry %s: %w", entry.SourceGroupID, err)
		}
	}
	return nil
}

func (q *Queries) DeleteOSSUnionEntriesBySourceGroups(ctx context.Context, sourceGroupIDs []string) error {
	for _, sourceGroupID := range sourceGroupIDs {
		if _, err := q.db.Exec(ctx, `DELETE FROM oss_union_entries WHERE source_group_id = $1`, sourceGroupID); err != nil {
			return fmt.Errorf("delete oss union entry %s: %w", sourceGroupID, err)
		}
	}
	return nil
}

func (q *Queries) UpsertPeriodicSummaryEntries(ctx context.Context, entries []domain.PeriodicSummaryEntry) error {
	for _, entry := range entries {
		_, err := q.db.Exec(ctx, `
			INSERT INTO periodic_summary_entries (
				id, bokio_company_id, source_group_id, source_object_type, source_object_id, stripe_event_id,
				filing_period, buyer_vat_number, row_type, amount_sek_ore, exported_amount_sek,
				review_state, review_reason, payload
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
				$11, $12, $13, $14
			)
			ON CONFLICT (source_group_id) DO UPDATE SET
				bokio_company_id = EXCLUDED.bokio_company_id,
				source_object_type = EXCLUDED.source_object_type,
				source_object_id = EXCLUDED.source_object_id,
				stripe_event_id = EXCLUDED.stripe_event_id,
				filing_period = EXCLUDED.filing_period,
				buyer_vat_number = EXCLUDED.buyer_vat_number,
				row_type = EXCLUDED.row_type,
				amount_sek_ore = EXCLUDED.amount_sek_ore,
				exported_amount_sek = EXCLUDED.exported_amount_sek,
				review_state = EXCLUDED.review_state,
				review_reason = EXCLUDED.review_reason,
				payload = EXCLUDED.payload,
				updated_at = now()
		`,
			entry.ID,
			entry.BokioCompanyID,
			entry.SourceGroupID,
			entry.SourceObjectType,
			entry.SourceObjectID,
			entry.StripeEventID,
			entry.FilingPeriod,
			entry.BuyerVATNumber,
			entry.RowType,
			entry.AmountSEKOre,
			entry.ExportedAmountSEK,
			entry.ReviewState,
			entry.ReviewReason,
			[]byte(entry.Payload),
		)
		if err != nil {
			return fmt.Errorf("upsert periodic summary entry %s: %w", entry.SourceGroupID, err)
		}
	}
	return nil
}

func (q *Queries) DeletePeriodicSummaryEntriesBySourceGroups(ctx context.Context, sourceGroupIDs []string) error {
	for _, sourceGroupID := range sourceGroupIDs {
		if _, err := q.db.Exec(ctx, `DELETE FROM periodic_summary_entries WHERE source_group_id = $1`, sourceGroupID); err != nil {
			return fmt.Errorf("delete periodic summary entry %s: %w", sourceGroupID, err)
		}
	}
	return nil
}

func (q *Queries) ListOSSUnionEntriesByPeriod(ctx context.Context, companyID uuid.UUID, period string) ([]domain.OSSUnionEntry, error) {
	rows, err := q.db.Query(ctx, `
		SELECT id, bokio_company_id, source_group_id, source_object_type, source_object_id, stripe_event_id,
			original_supply_period, filing_period, correction_target_period, consumption_country,
			origin_country, origin_identifier, sale_type, vat_rate_basis_points,
			taxable_amount_eur_cents, vat_amount_eur_cents, review_state, review_reason, payload,
			created_at, updated_at
		FROM oss_union_entries
		WHERE bokio_company_id = $1 AND filing_period = $2
		ORDER BY source_group_id
	`, companyID, period)
	if err != nil {
		return nil, fmt.Errorf("list oss union entries by period: %w", err)
	}
	defer rows.Close()

	var entries []domain.OSSUnionEntry
	for rows.Next() {
		entry, err := scanOSSUnionEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate oss union entries: %w", err)
	}
	return entries, nil
}

func (q *Queries) ListPeriodicSummaryEntriesByPeriod(ctx context.Context, companyID uuid.UUID, period string) ([]domain.PeriodicSummaryEntry, error) {
	rows, err := q.db.Query(ctx, `
		SELECT id, bokio_company_id, source_group_id, source_object_type, source_object_id, stripe_event_id,
			filing_period, buyer_vat_number, row_type, amount_sek_ore, exported_amount_sek,
			review_state, review_reason, payload, created_at, updated_at
		FROM periodic_summary_entries
		WHERE bokio_company_id = $1 AND filing_period = $2
		ORDER BY buyer_vat_number, row_type, source_group_id
	`, companyID, period)
	if err != nil {
		return nil, fmt.Errorf("list periodic summary entries by period: %w", err)
	}
	defer rows.Close()

	var entries []domain.PeriodicSummaryEntry
	for rows.Next() {
		entry, err := scanPeriodicSummaryEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate periodic summary entries: %w", err)
	}
	return entries, nil
}
