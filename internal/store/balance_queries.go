package store

import (
	"context"
	"fmt"

	"github.com/ecleangg/booky/internal/domain"
)

func (q *Queries) UpsertBalanceTransaction(ctx context.Context, bt domain.BalanceTransaction) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO stripe_balance_transactions (
			stripe_balance_transaction_id, stripe_account_id, source_object_type, source_object_id,
			type, reporting_category, status, currency, currency_exponent, amount_minor, fee_minor,
			net_minor, exchange_rate, amount_sek_ore, fee_sek_ore, net_sek_ore, occurred_at,
			available_on, payout_id, source_event_id, payload
		) VALUES (
			$1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, NULLIF($6, ''), NULLIF($7, ''), $8, $9,
			$10, $11, $12, $13, $14, $15, $16, $17, $18, NULLIF($19, ''), NULLIF($20, ''), $21
		)
		ON CONFLICT (stripe_balance_transaction_id) DO UPDATE SET
			stripe_account_id = EXCLUDED.stripe_account_id,
			source_object_type = EXCLUDED.source_object_type,
			source_object_id = EXCLUDED.source_object_id,
			type = EXCLUDED.type,
			reporting_category = EXCLUDED.reporting_category,
			status = EXCLUDED.status,
			currency = EXCLUDED.currency,
			currency_exponent = EXCLUDED.currency_exponent,
			amount_minor = EXCLUDED.amount_minor,
			fee_minor = EXCLUDED.fee_minor,
			net_minor = EXCLUDED.net_minor,
			exchange_rate = EXCLUDED.exchange_rate,
			amount_sek_ore = EXCLUDED.amount_sek_ore,
			fee_sek_ore = EXCLUDED.fee_sek_ore,
			net_sek_ore = EXCLUDED.net_sek_ore,
			occurred_at = EXCLUDED.occurred_at,
			available_on = EXCLUDED.available_on,
			payout_id = EXCLUDED.payout_id,
			source_event_id = EXCLUDED.source_event_id,
			payload = EXCLUDED.payload,
			updated_at = now()
	`,
		bt.ID,
		bt.StripeAccountID,
		bt.SourceObjectType,
		bt.SourceObjectID,
		bt.Type,
		bt.ReportingCategory,
		bt.Status,
		bt.Currency,
		bt.CurrencyExponent,
		bt.AmountMinor,
		bt.FeeMinor,
		bt.NetMinor,
		bt.ExchangeRate,
		bt.AmountSEKOre,
		bt.FeeSEKOre,
		bt.NetSEKOre,
		bt.OccurredAt,
		bt.AvailableOn,
		bt.PayoutID,
		bt.SourceEventID,
		[]byte(bt.Payload),
	)
	if err != nil {
		return fmt.Errorf("upsert balance transaction: %w", err)
	}
	return nil
}
