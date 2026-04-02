package stripe

import (
	"context"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/store"
	"github.com/google/uuid"
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

	result, err := s.buildFromEvent(ctx, evt)
	if err != nil {
		_ = s.Repo.InTx(ctx, func(q *store.Queries) error { return q.MarkWebhookFailed(ctx, evt.ID, err.Error()) })
		s.notifyWebhookFailure(ctx, evt, err)
		return err
	}

	if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
		for _, snapshot := range result.Snapshots {
			if err := q.UpsertObjectSnapshot(ctx, snapshot); err != nil {
				return err
			}
		}
		persistedCaseIDs := make(map[string]uuid.UUID, len(result.TaxCases))
		for _, taxCase := range result.TaxCases {
			persistedID, err := q.UpsertTaxCase(ctx, taxCase)
			if err != nil {
				return err
			}
			persistedCaseIDs[taxCase.RootObjectType+":"+taxCase.RootObjectID] = persistedID
		}
		for _, taxCase := range result.TaxCases {
			var objects []domain.TaxCaseObject
			for _, object := range result.TaxCaseObjects {
				if object.TaxCaseID == taxCase.ID {
					objects = append(objects, object)
				}
			}
			persistedID := taxCase.ID
			if existingID, ok := persistedCaseIDs[taxCase.RootObjectType+":"+taxCase.RootObjectID]; ok {
				persistedID = existingID
			}
			if err := q.ReplaceTaxCaseObjects(ctx, persistedID, objects); err != nil {
				return err
			}
		}
		rebindFactTaxCaseIDs(result.Facts, result.TaxCases, persistedCaseIDs)
		for _, bt := range result.BalanceTxs {
			if err := q.UpsertBalanceTransaction(ctx, bt); err != nil {
				return err
			}
		}
		if len(result.Facts) > 0 {
			if err := q.UpsertAccountingFacts(ctx, result.Facts); err != nil {
				return err
			}
		}
		return q.MarkWebhookProcessed(ctx, evt.ID)
	}); err != nil {
		s.notifyWebhookFailure(ctx, evt, err)
		return err
	}
	if s.Filings != nil && s.Filings.Enabled() {
		if err := s.Filings.SyncWebhookEntries(ctx, result.TaxCases, result.Facts); err != nil && s.Logger != nil {
			s.Logger.ErrorContext(ctx, "sync filing entries from webhook", "event_id", evt.ID, "error", err)
		}
	}

	s.notifyWebhookReviewFacts(ctx, evt, result.Facts)
	s.Logger.InfoContext(ctx, "stripe event ingested", "event_id", evt.ID, "event_type", evt.Type, "fact_count", len(result.Facts))
	return nil
}

func rebindFactTaxCaseIDs(facts []domain.AccountingFact, taxCases []domain.TaxCase, persistedCaseIDs map[string]uuid.UUID) {
	for i := range facts {
		if facts[i].TaxCaseID == nil {
			continue
		}
		for _, taxCase := range taxCases {
			if *facts[i].TaxCaseID != taxCase.ID {
				continue
			}
			if persistedID, ok := persistedCaseIDs[taxCase.RootObjectType+":"+taxCase.RootObjectID]; ok {
				facts[i].TaxCaseID = &persistedID
			}
			break
		}
	}
}

func (s *Service) buildFromEvent(ctx context.Context, evt Event) (ingestResult, error) {
	switch evt.Type {
	case "charge.succeeded", "charge.updated":
		return s.handleCharge(ctx, evt)
	case "payment_intent.succeeded":
		return s.handlePaymentIntentSucceeded(ctx, evt)
	case "checkout.session.completed", "checkout.session.async_payment_succeeded":
		return s.handleCheckoutSession(ctx, evt)
	case "invoice.paid", "invoice.payment_succeeded":
		return s.handleInvoice(ctx, evt)
	case "charge.refunded":
		return s.handleRefundedCharge(ctx, evt)
	case "refund.created":
		return s.handleRefund(ctx, evt)
	case "customer.tax_id.updated":
		return s.handleCustomerTaxIDUpdated(ctx, evt)
	case "charge.dispute.created":
		return s.handleDisputeCreated(ctx, evt)
	case "payout.paid":
		return s.handlePayout(ctx, evt)
	default:
		return ingestResult{}, nil
	}
}
