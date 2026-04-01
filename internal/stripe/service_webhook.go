package stripe

import (
	"context"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/store"
)

func (s *Service) HandleWebhook(ctx context.Context, payload []byte, signatureHeader string) error {
	if err := s.Client.VerifyWebhook(payload, signatureHeader); err != nil {
		return err
	}

	evt, err := s.Client.ParseEvent(payload)
	if err != nil {
		return err
	}

	inserted := false
	eventRecord := domain.StripeWebhookEvent{
		ID:              evt.ID,
		EventType:       evt.Type,
		Livemode:        evt.Livemode,
		APIVersion:      evt.APIVersion,
		StripeCreatedAt: time.Unix(evt.Created, 0).UTC(),
		Payload:         payload,
	}
	if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
		var err error
		inserted, err = q.InsertWebhookEvent(ctx, eventRecord)
		return err
	}); err != nil {
		return err
	}
	if !inserted {
		existing, err := s.Repo.Queries().GetWebhookEvent(ctx, evt.ID)
		if err != nil {
			return err
		}
		if existing.ProcessedAt != nil && existing.ProcessingError == nil {
			return nil
		}
		if err := s.Repo.InTx(ctx, func(q *store.Queries) error { return q.ResetWebhookForRetry(ctx, eventRecord) }); err != nil {
			return err
		}
	}

	snapshots, balanceTransactions, facts, err := s.buildFromEvent(ctx, evt)
	if err != nil {
		_ = s.Repo.InTx(ctx, func(q *store.Queries) error { return q.MarkWebhookFailed(ctx, evt.ID, err.Error()) })
		s.notifyWebhookFailure(ctx, evt, err)
		return err
	}

	if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
		for _, snapshot := range snapshots {
			if err := q.UpsertObjectSnapshot(ctx, snapshot); err != nil {
				return err
			}
		}
		for _, bt := range balanceTransactions {
			if err := q.UpsertBalanceTransaction(ctx, bt); err != nil {
				return err
			}
		}
		if len(facts) > 0 {
			if err := q.UpsertAccountingFacts(ctx, facts); err != nil {
				return err
			}
		}
		return q.MarkWebhookProcessed(ctx, evt.ID)
	}); err != nil {
		s.notifyWebhookFailure(ctx, evt, err)
		return err
	}
	if s.Filings != nil && s.Filings.Enabled() {
		if err := s.Filings.SyncWebhookEntries(ctx, snapshots, facts); err != nil && s.Logger != nil {
			s.Logger.ErrorContext(ctx, "sync filing entries from webhook", "event_id", evt.ID, "error", err)
		}
	}

	s.notifyWebhookReviewFacts(ctx, evt, facts)
	s.Logger.InfoContext(ctx, "stripe event ingested", "event_id", evt.ID, "event_type", evt.Type, "fact_count", len(facts))
	return nil
}

func (s *Service) buildFromEvent(ctx context.Context, evt Event) ([]domain.ObjectSnapshot, []domain.BalanceTransaction, []domain.AccountingFact, error) {
	switch evt.Type {
	case "charge.succeeded", "charge.updated":
		return s.handleCharge(ctx, evt)
	case "payment_intent.succeeded":
		return s.handlePaymentIntentSucceeded(ctx, evt)
	case "charge.refunded":
		return s.handleRefundedCharge(ctx, evt)
	case "charge.dispute.created":
		return s.handleDisputeCreated(ctx, evt)
	case "payout.paid":
		return s.handlePayout(ctx, evt)
	default:
		return nil, nil, nil, nil
	}
}
