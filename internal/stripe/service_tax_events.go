package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ecleangg/booky/internal/accounting"
	"github.com/ecleangg/booky/internal/domain"
	"github.com/google/uuid"
)

func (s *Service) handleCheckoutSession(ctx context.Context, evt Event) (ingestResult, error) {
	var session CheckoutSession
	if err := json.Unmarshal(evt.Data.Object, &session); err != nil {
		return ingestResult{}, fmt.Errorf("decode checkout session: %w", err)
	}
	fullSession, raw, err := s.Client.GetCheckoutSession(ctx, session.ID)
	if err != nil {
		return ingestResult{}, err
	}
	snapshots := []domain.ObjectSnapshot{{ObjectType: "checkout_session", ObjectID: fullSession.ID, Livemode: evt.Livemode, Payload: raw}}
	more, err := s.saleSnapshotsForCheckoutSession(ctx, evt, fullSession)
	if err != nil {
		return ingestResult{}, err
	}
	snapshots = dedupeSnapshots(append(snapshots, more...))
	taxCase, objects, err := s.buildTaxCase(ctx, evt.Livemode, snapshots)
	if err != nil {
		return ingestResult{}, err
	}

	if strings.TrimSpace(fullSession.PaymentIntent) == "" {
		return ingestResult{Snapshots: snapshots, TaxCases: []domain.TaxCase{taxCase}, TaxCaseObjects: objects}, nil
	}
	pi, _, err := s.Client.GetPaymentIntent(ctx, fullSession.PaymentIntent)
	if err != nil {
		return ingestResult{}, err
	}
	if pi.LatestCharge.ID == "" {
		return ingestResult{Snapshots: snapshots, TaxCases: []domain.TaxCase{taxCase}, TaxCaseObjects: objects}, nil
	}
	charge, _, err := s.getChargeReadyForAccounting(ctx, pi.LatestCharge.ID)
	if err != nil {
		return ingestResult{}, err
	}
	balanceTxID := extractID(charge.BalanceTransaction)
	if balanceTxID == "" {
		return ingestResult{Snapshots: snapshots, TaxCases: []domain.TaxCase{taxCase}, TaxCaseObjects: objects}, nil
	}
	btAPI, btRaw, err := s.Client.GetBalanceTransaction(ctx, balanceTxID)
	if err != nil {
		return ingestResult{}, err
	}
	bt := convertBalanceTransaction(s.Config, evt, btAPI, btRaw)
	facts, err := s.buildChargeFacts(evt, charge, bt, taxCase)
	if err != nil {
		return ingestResult{}, err
	}
	snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "balance_transaction", ObjectID: bt.ID, Livemode: evt.Livemode, Payload: btRaw})
	return ingestResult{
		Snapshots:      dedupeSnapshots(snapshots),
		BalanceTxs:     []domain.BalanceTransaction{bt},
		TaxCases:       []domain.TaxCase{taxCase},
		TaxCaseObjects: objects,
		Facts:          facts,
	}, nil
}

func (s *Service) handleInvoice(ctx context.Context, evt Event) (ingestResult, error) {
	var eventInvoice Invoice
	if err := json.Unmarshal(evt.Data.Object, &eventInvoice); err != nil {
		return ingestResult{}, fmt.Errorf("decode invoice: %w", err)
	}
	invoice, raw, err := s.Client.GetInvoice(ctx, eventInvoice.ID)
	if err != nil {
		return ingestResult{}, err
	}
	snapshots := []domain.ObjectSnapshot{{ObjectType: "invoice", ObjectID: invoice.ID, Livemode: evt.Livemode, Payload: raw}}
	if invoice.Subscription != "" {
		sessions, raws, err := s.Client.ListCheckoutSessionsBySubscription(ctx, invoice.Subscription)
		if err != nil {
			return ingestResult{}, err
		}
		for i, session := range sessions {
			snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "checkout_session", ObjectID: session.ID, Livemode: evt.Livemode, Payload: raws[i]})
		}
	}
	if invoice.Customer != "" {
		customer, customerRaw, err := s.Client.GetCustomer(ctx, invoice.Customer)
		if err != nil {
			return ingestResult{}, err
		}
		snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "customer", ObjectID: customer.ID, Livemode: evt.Livemode, Payload: customerRaw})
		taxIDs, raws, err := s.Client.ListCustomerTaxIDs(ctx, invoice.Customer)
		if err != nil {
			return ingestResult{}, err
		}
		for i, item := range taxIDs {
			snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "tax_id", ObjectID: item.ID, Livemode: evt.Livemode, Payload: raws[i]})
		}
	}
	taxCase, objects, err := s.buildTaxCase(ctx, evt.Livemode, snapshots)
	if err != nil {
		return ingestResult{}, err
	}
	return ingestResult{Snapshots: dedupeSnapshots(snapshots), TaxCases: []domain.TaxCase{taxCase}, TaxCaseObjects: objects}, nil
}

func (s *Service) handleRefund(ctx context.Context, evt Event) (ingestResult, error) {
	var eventRefund Refund
	if err := json.Unmarshal(evt.Data.Object, &eventRefund); err != nil {
		return ingestResult{}, fmt.Errorf("decode refund: %w", err)
	}
	refund, refundRaw, err := s.Client.GetRefund(ctx, eventRefund.ID)
	if err != nil {
		return ingestResult{}, err
	}
	if strings.TrimSpace(refund.Charge) == "" {
		return ingestResult{Snapshots: []domain.ObjectSnapshot{{ObjectType: "refund", ObjectID: refund.ID, Livemode: evt.Livemode, Payload: refundRaw}}}, nil
	}
	charge, chargeRaw, err := s.getChargeReadyForAccounting(ctx, refund.Charge)
	if err != nil {
		return ingestResult{}, err
	}
	snapshots := []domain.ObjectSnapshot{
		{ObjectType: "refund", ObjectID: refund.ID, Livemode: evt.Livemode, Payload: refundRaw},
		{ObjectType: "charge", ObjectID: charge.ID, Livemode: evt.Livemode, Payload: chargeRaw},
	}
	snapshots, err = s.saleSnapshotsForCharge(ctx, evt, charge, snapshots...)
	if err != nil {
		return ingestResult{}, err
	}
	taxCase, objects, err := s.buildTaxCase(ctx, evt.Livemode, snapshots)
	if err != nil {
		return ingestResult{}, err
	}

	refundBTID := extractID(refund.BalanceTransaction)
	if refundBTID == "" {
		return ingestResult{Snapshots: snapshots, TaxCases: []domain.TaxCase{taxCase}, TaxCaseObjects: objects}, nil
	}
	chargeBTID := extractID(charge.BalanceTransaction)
	if chargeBTID == "" {
		return ingestResult{Snapshots: snapshots, TaxCases: []domain.TaxCase{taxCase}, TaxCaseObjects: objects}, nil
	}
	chargeBTAPI, chargeBTRaw, err := s.Client.GetBalanceTransaction(ctx, chargeBTID)
	if err != nil {
		return ingestResult{}, err
	}
	primaryBT := convertBalanceTransaction(s.Config, evt, chargeBTAPI, chargeBTRaw)
	refundBTAPI, refundBTRaw, err := s.Client.GetBalanceTransaction(ctx, refundBTID)
	if err != nil {
		return ingestResult{}, err
	}
	refundBT := convertBalanceTransaction(s.Config, evt, refundBTAPI, refundBTRaw)
	resolution := accounting.ResolveTaxCase(s.Config, taxCase, settledGrossSEKOre(primaryBT), explicitVATFromTaxCase(taxCase, primaryBT))
	grossSEK := settledGrossSEKOre(refundBT)
	if grossSEK == 0 && strings.ToUpper(refund.Currency) != "SEK" {
		obs, err := accounting.BuildReviewNoteFact(s.Config.Bokio.CompanyID, stripeAccountID(evt), "refund:"+refund.ID, "refund", refund.ID, refundBT.ID, evt.ID, s.postingTime(refund.Created), strings.ToUpper(refund.Currency), refund.Amount, s.Config.Accounts.FallbackOBS, "refund missing settled SEK amount", refund)
		if err != nil {
			return ingestResult{}, err
		}
		return ingestResult{
			Snapshots:      append(snapshots, domain.ObjectSnapshot{ObjectType: "balance_transaction", ObjectID: primaryBT.ID, Livemode: evt.Livemode, Payload: chargeBTRaw}, domain.ObjectSnapshot{ObjectType: "balance_transaction", ObjectID: refundBT.ID, Livemode: evt.Livemode, Payload: refundBTRaw}),
			BalanceTxs:     []domain.BalanceTransaction{primaryBT, refundBT},
			TaxCases:       []domain.TaxCase{taxCase},
			TaxCaseObjects: objects,
			Facts:          []domain.AccountingFact{obs},
		}, nil
	}

	var facts []domain.AccountingFact
	if resolution.ReviewReason != "" {
		reviewFacts, err := accounting.BuildReviewTransferFacts(accounting.ReviewTransferInput{
			BokioCompanyID:             s.Config.Bokio.CompanyID,
			StripeAccountID:            stripeAccountID(evt),
			TaxCaseID:                  &taxCase.ID,
			SourceGroupID:              "refund:" + refund.ID,
			SourceObjectType:           "refund",
			SourceObjectID:             refund.ID,
			StripeBalanceTransactionID: refundBT.ID,
			StripeEventID:              evt.ID,
			PostingDate:                s.postingTime(refund.Created),
			SourceCurrency:             strings.ToUpper(refund.Currency),
			SourceAmountMinor:          refund.Amount,
			AmountSEKOre:               grossSEK,
			DebitFactType:              "refund_review_obs",
			CreditFactType:             "refund_review_receivable",
			DebitAccount:               s.Config.Accounts.FallbackOBS,
			CreditAccount:              s.Config.Accounts.StripeReceivable,
			ReviewReason:               resolution.ReviewReason,
			Payload:                    refund,
		})
		if err != nil {
			return ingestResult{}, err
		}
		facts = append(facts, reviewFacts...)
	} else {
		vatSEK := proportionalAmount(resolution.VATSEKOre, refund.Amount, charge.Amount)
		revenueSEK := grossSEK - vatSEK
		refundFacts, err := accounting.BuildRefundFacts(accounting.RefundInput{
			BokioCompanyID:             s.Config.Bokio.CompanyID,
			StripeAccountID:            stripeAccountID(evt),
			TaxCaseID:                  &taxCase.ID,
			SourceObjectID:             refund.ID,
			SourceGroupID:              "refund:" + refund.ID,
			StripeBalanceTransactionID: refundBT.ID,
			StripeEventID:              evt.ID,
			PostingDate:                s.postingTime(refund.Created),
			SourceCurrency:             strings.ToUpper(refund.Currency),
			GrossMinor:                 refund.Amount,
			GrossSEKOre:                grossSEK,
			MarketCode:                 resolution.MarketCode,
			VATTreatment:               resolution.VATTreatment,
			RevenueAccount:             resolution.RevenueAccount,
			OutputVATAccount:           resolution.OutputVATAccount,
			ReceivableAccount:          s.Config.Accounts.StripeReceivable,
			RevenueSEKOre:              revenueSEK,
			VATSEKOre:                  vatSEK,
			Payload:                    refund,
		})
		if err != nil {
			return ingestResult{}, err
		}
		facts = append(facts, refundFacts...)
	}
	return ingestResult{
		Snapshots: append(snapshots,
			domain.ObjectSnapshot{ObjectType: "balance_transaction", ObjectID: primaryBT.ID, Livemode: evt.Livemode, Payload: chargeBTRaw},
			domain.ObjectSnapshot{ObjectType: "balance_transaction", ObjectID: refundBT.ID, Livemode: evt.Livemode, Payload: refundBTRaw},
		),
		BalanceTxs:     []domain.BalanceTransaction{primaryBT, refundBT},
		TaxCases:       []domain.TaxCase{taxCase},
		TaxCaseObjects: objects,
		Facts:          facts,
	}, nil
}

func (s *Service) handleCustomerTaxIDUpdated(ctx context.Context, evt Event) (ingestResult, error) {
	var updated TaxID
	if err := json.Unmarshal(evt.Data.Object, &updated); err != nil {
		return ingestResult{}, fmt.Errorf("decode customer tax id: %w", err)
	}
	snapshots := []domain.ObjectSnapshot{{ObjectType: "tax_id", ObjectID: updated.ID, Livemode: evt.Livemode, Payload: evt.Data.Object}}
	if strings.TrimSpace(updated.Customer) == "" {
		return ingestResult{Snapshots: snapshots}, nil
	}

	customer, customerRaw, err := s.Client.GetCustomer(ctx, updated.Customer)
	if err != nil {
		return ingestResult{}, err
	}
	snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "customer", ObjectID: customer.ID, Livemode: evt.Livemode, Payload: customerRaw})
	taxIDs, raws, err := s.Client.ListCustomerTaxIDs(ctx, updated.Customer)
	if err != nil {
		return ingestResult{}, err
	}
	for i, item := range taxIDs {
		snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "tax_id", ObjectID: item.ID, Livemode: evt.Livemode, Payload: raws[i]})
	}

	relatedCases, err := s.Repo.Queries().ListTaxCasesByObject(ctx, "customer", updated.Customer)
	if err != nil {
		return ingestResult{}, err
	}
	var taxCases []domain.TaxCase
	var caseObjects []domain.TaxCaseObject
	var caseIDs []uuid.UUID
	for _, existing := range relatedCases {
		caseIDs = append(caseIDs, existing.ID)
		objects, err := s.Repo.Queries().ListTaxCaseObjects(ctx, existing.ID)
		if err != nil {
			return ingestResult{}, err
		}
		caseSnapshots := append([]domain.ObjectSnapshot{}, snapshots...)
		for _, object := range objects {
			if object.ObjectType == "customer" || object.ObjectType == "tax_id" {
				continue
			}
			snapshot, err := s.Repo.Queries().GetObjectSnapshot(ctx, object.ObjectType, object.ObjectID)
			if err != nil {
				continue
			}
			caseSnapshots = append(caseSnapshots, snapshot)
		}
		rebuilt, objects, err := s.buildTaxCase(ctx, existing.Livemode, caseSnapshots)
		if err != nil {
			return ingestResult{}, err
		}
		rebuilt.ID = existing.ID
		for i := range objects {
			objects[i].TaxCaseID = existing.ID
		}
		taxCases = append(taxCases, rebuilt)
		caseObjects = append(caseObjects, objects...)
	}
	facts, err := s.Repo.Queries().ListFactsByTaxCaseIDs(ctx, caseIDs)
	if err != nil {
		return ingestResult{}, err
	}
	return ingestResult{Snapshots: dedupeSnapshots(snapshots), TaxCases: taxCases, TaxCaseObjects: caseObjects, Facts: facts}, nil
}
