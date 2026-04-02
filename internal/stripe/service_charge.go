package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/ecleangg/booky/internal/accounting"
	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/support"
)

func (s *Service) loadChargeEvidence(ctx context.Context, evt Event, charge Charge) (chargeEvidenceBundle, []domain.ObjectSnapshot, error) {
	var bundle chargeEvidenceBundle
	var snapshots []domain.ObjectSnapshot

	if charge.Invoice != "" {
		invoice, invoiceRaw, err := s.Client.GetInvoice(ctx, charge.Invoice)
		if err != nil {
			return chargeEvidenceBundle{}, nil, err
		}
		bundle.Invoice = &invoice
		snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "invoice", ObjectID: invoice.ID, Livemode: evt.Livemode, Payload: invoiceRaw})
	}

	customerID := strings.TrimSpace(charge.Customer)
	if customerID == "" && bundle.Invoice != nil {
		customerID = strings.TrimSpace(bundle.Invoice.Customer)
	}
	if customerID == "" {
		return bundle, snapshots, nil
	}

	customer, customerRaw, err := s.Client.GetCustomer(ctx, customerID)
	if err != nil {
		return chargeEvidenceBundle{}, nil, err
	}
	bundle.Customer = &customer
	snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "customer", ObjectID: customer.ID, Livemode: evt.Livemode, Payload: customerRaw})

	taxIDs, taxIDRaws, err := s.Client.ListCustomerTaxIDs(ctx, customerID)
	if err != nil {
		return chargeEvidenceBundle{}, nil, err
	}
	bundle.CustomerTaxIDs = taxIDs
	for i, taxID := range taxIDs {
		snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "tax_id", ObjectID: taxID.ID, Livemode: evt.Livemode, Payload: taxIDRaws[i]})
	}

	return bundle, snapshots, nil
}

func (s *Service) handlePaymentIntentSucceeded(ctx context.Context, evt Event) ([]domain.ObjectSnapshot, []domain.BalanceTransaction, []domain.AccountingFact, error) {
	var paymentIntent PaymentIntent
	if err := json.Unmarshal(evt.Data.Object, &paymentIntent); err != nil {
		return nil, nil, nil, fmt.Errorf("decode payment intent from event: %w", err)
	}

	fullPI, piRaw, err := s.Client.GetPaymentIntent(ctx, paymentIntent.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	snapshots := []domain.ObjectSnapshot{{ObjectType: "payment_intent", ObjectID: fullPI.ID, Livemode: evt.Livemode, Payload: piRaw}}
	if fullPI.LatestCharge.ID == "" {
		s.Logger.InfoContext(ctx, "payment intent has no latest charge yet", "event_id", evt.ID, "payment_intent_id", fullPI.ID)
		return snapshots, nil, nil, nil
	}

	charge, chargeRaw, err := s.getChargeReadyForAccounting(ctx, fullPI.LatestCharge.ID)
	if err != nil {
		return snapshots, nil, nil, err
	}
	evidenceBundle, evidenceSnapshots, err := s.loadChargeEvidence(ctx, evt, charge)
	if err != nil {
		return snapshots, nil, nil, err
	}
	balanceTxID := extractID(charge.BalanceTransaction)
	snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "charge", ObjectID: charge.ID, Livemode: evt.Livemode, Payload: chargeRaw})
	snapshots = append(snapshots, evidenceSnapshots...)
	if balanceTxID == "" {
		s.Logger.InfoContext(ctx, "payment intent latest charge has no balance transaction yet", "event_id", evt.ID, "payment_intent_id", fullPI.ID, "charge_id", charge.ID)
		return snapshots, nil, nil, nil
	}

	btAPI, btRaw, err := s.Client.GetBalanceTransaction(ctx, balanceTxID)
	if err != nil {
		return snapshots, nil, nil, err
	}
	bt := convertBalanceTransaction(s.Config, evt, btAPI, btRaw)
	facts, err := s.buildChargeFacts(evt, charge, bt, evidenceBundle)
	if err != nil {
		return snapshots, nil, nil, err
	}
	snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "balance_transaction", ObjectID: bt.ID, Livemode: evt.Livemode, Payload: btRaw})
	return snapshots, []domain.BalanceTransaction{bt}, facts, nil
}

func (s *Service) handleCharge(ctx context.Context, evt Event) ([]domain.ObjectSnapshot, []domain.BalanceTransaction, []domain.AccountingFact, error) {
	var eventCharge Charge
	if err := json.Unmarshal(evt.Data.Object, &eventCharge); err != nil {
		return nil, nil, nil, fmt.Errorf("decode charge from event: %w", err)
	}
	charge, chargeRaw, err := s.Client.GetCharge(ctx, eventCharge.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	charge, chargeRaw, err = s.getChargeReadyForAccounting(ctx, charge.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	evidenceBundle, evidenceSnapshots, err := s.loadChargeEvidence(ctx, evt, charge)
	if err != nil {
		return nil, nil, nil, err
	}
	balanceTxID := extractID(charge.BalanceTransaction)
	snapshots := []domain.ObjectSnapshot{{ObjectType: "charge", ObjectID: charge.ID, Livemode: evt.Livemode, Payload: chargeRaw}}
	snapshots = append(snapshots, evidenceSnapshots...)
	if balanceTxID == "" {
		s.Logger.InfoContext(ctx, "charge missing balance transaction, deferring accounting facts", "event_id", evt.ID, "charge_id", charge.ID)
		return snapshots, nil, nil, nil
	}
	btAPI, btRaw, err := s.Client.GetBalanceTransaction(ctx, balanceTxID)
	if err != nil {
		return snapshots, nil, nil, err
	}
	bt := convertBalanceTransaction(s.Config, evt, btAPI, btRaw)

	saleFacts, err := s.buildChargeFacts(evt, charge, bt, evidenceBundle)
	if err != nil {
		return snapshots, nil, nil, err
	}

	snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "balance_transaction", ObjectID: bt.ID, Livemode: evt.Livemode, Payload: btRaw})
	return snapshots, []domain.BalanceTransaction{bt}, saleFacts, nil
}

func (s *Service) handleRefundedCharge(ctx context.Context, evt Event) ([]domain.ObjectSnapshot, []domain.BalanceTransaction, []domain.AccountingFact, error) {
	var eventCharge Charge
	if err := json.Unmarshal(evt.Data.Object, &eventCharge); err != nil {
		return nil, nil, nil, fmt.Errorf("decode refunded charge: %w", err)
	}
	charge, chargeRaw, err := s.Client.GetCharge(ctx, eventCharge.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	charge, chargeRaw, err = s.getChargeReadyForAccounting(ctx, charge.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	evidenceBundle, evidenceSnapshots, err := s.loadChargeEvidence(ctx, evt, charge)
	if err != nil {
		return nil, nil, nil, err
	}
	chargeBTID := extractID(charge.BalanceTransaction)
	if chargeBTID == "" {
		snapshots := []domain.ObjectSnapshot{{ObjectType: "charge", ObjectID: charge.ID, Livemode: evt.Livemode, Payload: chargeRaw}}
		snapshots = append(snapshots, evidenceSnapshots...)
		s.Logger.InfoContext(ctx, "refunded charge missing primary balance transaction, deferring accounting facts", "event_id", evt.ID, "charge_id", charge.ID)
		return snapshots, nil, nil, nil
	}
	chargeBT, chargeBTRaw, err := s.Client.GetBalanceTransaction(ctx, chargeBTID)
	if err != nil {
		return nil, nil, nil, err
	}
	primaryBT := convertBalanceTransaction(s.Config, evt, chargeBT, chargeBTRaw)
	settledGross := settledGrossSEKOre(primaryBT)
	if settledGross == 0 && strings.ToUpper(charge.Currency) != "SEK" {
		obs, err := accounting.BuildReviewNoteFact(s.Config.Bokio.CompanyID, stripeAccountID(evt), "charge:"+charge.ID+":sale", "charge", charge.ID, primaryBT.ID, evt.ID, s.postingTime(charge.Created), strings.ToUpper(charge.Currency), charge.Amount, s.Config.Accounts.FallbackOBS, "charge missing settled SEK amount", charge)
		if err != nil {
			return nil, nil, nil, err
		}
		snapshots := []domain.ObjectSnapshot{{ObjectType: "charge", ObjectID: charge.ID, Livemode: evt.Livemode, Payload: chargeRaw}}
		snapshots = append(snapshots, evidenceSnapshots...)
		snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "balance_transaction", ObjectID: primaryBT.ID, Livemode: evt.Livemode, Payload: chargeBTRaw})
		return snapshots, []domain.BalanceTransaction{primaryBT}, []domain.AccountingFact{obs}, nil
	}
	resolution := accounting.ResolveSale(s.Config, buildSaleClassificationInput(charge, evidenceBundle, primaryBT))

	snapshots := []domain.ObjectSnapshot{{ObjectType: "charge", ObjectID: charge.ID, Livemode: evt.Livemode, Payload: chargeRaw}}
	snapshots = append(snapshots, evidenceSnapshots...)
	bts := []domain.BalanceTransaction{primaryBT}
	var facts []domain.AccountingFact
	for _, refund := range charge.Refunds.Data {
		refundBTID := extractID(refund.BalanceTransaction)
		if refundBTID == "" {
			continue
		}
		refundBTAPI, refundRaw, err := s.Client.GetBalanceTransaction(ctx, refundBTID)
		if err != nil {
			return nil, nil, nil, err
		}
		refundBT := convertBalanceTransaction(s.Config, evt, refundBTAPI, refundRaw)
		bts = append(bts, refundBT)
		snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "balance_transaction", ObjectID: refundBT.ID, Livemode: evt.Livemode, Payload: refundRaw})
		grossSEK := settledGrossSEKOre(refundBT)
		if grossSEK == 0 && strings.ToUpper(refund.Currency) != "SEK" {
			obs, err := accounting.BuildReviewNoteFact(s.Config.Bokio.CompanyID, stripeAccountID(evt), "refund:"+refund.ID, "refund", refund.ID, refundBT.ID, evt.ID, s.postingTime(refund.Created), strings.ToUpper(refund.Currency), refund.Amount, s.Config.Accounts.FallbackOBS, "refund missing settled SEK amount", refund)
			if err != nil {
				return nil, nil, nil, err
			}
			facts = append(facts, obs)
			continue
		}
		if resolution.ReviewReason != "" {
			reviewFacts, err := accounting.BuildReviewTransferFacts(accounting.ReviewTransferInput{
				BokioCompanyID:             s.Config.Bokio.CompanyID,
				StripeAccountID:            stripeAccountID(evt),
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
				return nil, nil, nil, err
			}
			facts = append(facts, reviewFacts...)
			continue
		}
		vatSEK := proportionalAmount(resolution.VATSEKOre, refund.Amount, charge.Amount)
		revenueSEK := grossSEK - vatSEK
		refundFacts, err := accounting.BuildRefundFacts(accounting.RefundInput{
			BokioCompanyID:             s.Config.Bokio.CompanyID,
			StripeAccountID:            stripeAccountID(evt),
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
			return nil, nil, nil, err
		}
		facts = append(facts, refundFacts...)
	}

	snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "balance_transaction", ObjectID: primaryBT.ID, Livemode: evt.Livemode, Payload: chargeBTRaw})
	return snapshots, bts, facts, nil
}

func (s *Service) buildChargeFacts(evt Event, charge Charge, bt domain.BalanceTransaction, evidenceBundle chargeEvidenceBundle) ([]domain.AccountingFact, error) {
	settledGross := settledGrossSEKOre(bt)
	if settledGross == 0 && strings.ToUpper(charge.Currency) != "SEK" {
		obs, err := accounting.BuildReviewNoteFact(s.Config.Bokio.CompanyID, stripeAccountID(evt), "charge:"+charge.ID+":sale", "charge", charge.ID, bt.ID, evt.ID, s.postingTime(charge.Created), strings.ToUpper(charge.Currency), charge.Amount, s.Config.Accounts.FallbackOBS, "charge missing settled SEK amount", charge)
		if err != nil {
			return nil, err
		}
		return []domain.AccountingFact{obs}, nil
	}
	resolution := accounting.ResolveSale(s.Config, buildSaleClassificationInput(charge, evidenceBundle, bt))
	if resolution.ReviewReason != "" {
		return s.buildChargeReviewFacts(evt, charge, bt, resolution.ReviewReason)
	}

	sourceCurrency := strings.ToUpper(charge.Currency)
	settlementCurrency := strings.ToUpper(bt.Currency)
	stripeBalanceAccount, ok := s.Config.Accounts.StripeBalanceByCurrency[settlementCurrency]
	if !ok {
		return s.buildChargeReviewFacts(evt, charge, bt, fmt.Sprintf("no Stripe balance account mapping for settlement currency %s", settlementCurrency))
	}

	feeSEK := settledFeeSEKOre(bt)
	feeReverseVAT := int64(math.Round(float64(feeSEK) * 0.25))
	return accounting.BuildSaleFacts(accounting.SaleInput{
		BokioCompanyID:             s.Config.Bokio.CompanyID,
		StripeAccountID:            stripeAccountID(evt),
		SourceObjectType:           "charge",
		SourceObjectID:             charge.ID,
		SourceGroupPrefix:          "charge:" + charge.ID,
		StripeBalanceTransactionID: bt.ID,
		StripeEventID:              evt.ID,
		ChargeDate:                 s.postingTime(charge.Created),
		AvailableOn:                bt.AvailableOn,
		SourceCurrency:             sourceCurrency,
		GrossMinor:                 charge.Amount,
		FeeCurrency:                settlementCurrency,
		FeeMinor:                   support.AbsInt64(bt.FeeMinor),
		GrossSEKOre:                settledGrossSEKOre(bt),
		FeeSEKOre:                  feeSEK,
		MarketCode:                 resolution.MarketCode,
		VATTreatment:               resolution.VATTreatment,
		RevenueAccount:             resolution.RevenueAccount,
		OutputVATAccount:           resolution.OutputVATAccount,
		ReceivableAccount:          s.Config.Accounts.StripeReceivable,
		StripeBalanceAccount:       stripeBalanceAccount,
		FeeExpenseAccount:          s.Config.Accounts.StripeFees.Expense,
		FeeInputVATAccount:         s.Config.Accounts.StripeFees.InputVAT,
		FeeOutputVATAccount:        s.Config.Accounts.StripeFees.OutputVAT,
		RevenueSEKOre:              resolution.RevenueSEKOre,
		VATSEKOre:                  resolution.VATSEKOre,
		FeeReverseVATSEKOre:        feeReverseVAT,
		Payload:                    charge,
	})
}

func (s *Service) buildChargeReviewFacts(evt Event, charge Charge, bt domain.BalanceTransaction, reason string) ([]domain.AccountingFact, error) {
	sourceCurrency := strings.ToUpper(charge.Currency)
	settlementCurrency := strings.ToUpper(bt.Currency)
	stripeBalanceAccount, ok := s.Config.Accounts.StripeBalanceByCurrency[settlementCurrency]
	if !ok {
		stripeBalanceAccount = s.Config.Accounts.FallbackOBS
	}

	var facts []domain.AccountingFact
	saleFacts, err := accounting.BuildReviewTransferFacts(accounting.ReviewTransferInput{
		BokioCompanyID:             s.Config.Bokio.CompanyID,
		StripeAccountID:            stripeAccountID(evt),
		SourceGroupID:              "charge:" + charge.ID + ":sale",
		SourceObjectType:           "charge",
		SourceObjectID:             charge.ID,
		StripeBalanceTransactionID: bt.ID,
		StripeEventID:              evt.ID,
		PostingDate:                s.postingTime(charge.Created),
		SourceCurrency:             sourceCurrency,
		SourceAmountMinor:          charge.Amount,
		AmountSEKOre:               settledGrossSEKOre(bt),
		DebitFactType:              "sale_review_receivable",
		CreditFactType:             "sale_review_obs",
		DebitAccount:               s.Config.Accounts.StripeReceivable,
		CreditAccount:              s.Config.Accounts.FallbackOBS,
		ReviewReason:               reason,
		Payload:                    charge,
	})
	if err != nil {
		return nil, err
	}
	facts = append(facts, saleFacts...)

	feeSEK := settledFeeSEKOre(bt)
	if feeSEK > 0 {
		feeFacts, err := accounting.BuildReviewTransferFacts(accounting.ReviewTransferInput{
			BokioCompanyID:             s.Config.Bokio.CompanyID,
			StripeAccountID:            stripeAccountID(evt),
			SourceGroupID:              "charge:" + charge.ID + ":fee",
			SourceObjectType:           "charge",
			SourceObjectID:             charge.ID,
			StripeBalanceTransactionID: bt.ID,
			StripeEventID:              evt.ID,
			PostingDate:                s.postingTime(charge.Created),
			SourceCurrency:             settlementCurrency,
			SourceAmountMinor:          support.AbsInt64(bt.FeeMinor),
			AmountSEKOre:               feeSEK,
			DebitFactType:              "stripe_fee_review_obs",
			CreditFactType:             "stripe_fee_review_balance",
			DebitAccount:               s.Config.Accounts.FallbackOBS,
			CreditAccount:              stripeBalanceAccount,
			ReviewReason:               reason,
			Payload:                    charge,
		})
		if err != nil {
			return nil, err
		}
		facts = append(facts, feeFacts...)
	}

	if bt.AvailableOn != nil {
		availableFacts, err := accounting.BuildReviewTransferFacts(accounting.ReviewTransferInput{
			BokioCompanyID:             s.Config.Bokio.CompanyID,
			StripeAccountID:            stripeAccountID(evt),
			SourceGroupID:              "charge:" + charge.ID + ":available",
			SourceObjectType:           "charge",
			SourceObjectID:             charge.ID,
			StripeBalanceTransactionID: bt.ID,
			StripeEventID:              evt.ID,
			PostingDate:                *bt.AvailableOn,
			SourceCurrency:             sourceCurrency,
			SourceAmountMinor:          charge.Amount,
			AmountSEKOre:               settledGrossSEKOre(bt),
			DebitFactType:              "stripe_available_review_debit",
			CreditFactType:             "stripe_available_review_credit",
			DebitAccount:               stripeBalanceAccount,
			CreditAccount:              s.Config.Accounts.StripeReceivable,
			ReviewReason:               reason,
			Payload:                    charge,
		})
		if err != nil {
			return nil, err
		}
		facts = append(facts, availableFacts...)
	}

	return facts, nil
}
