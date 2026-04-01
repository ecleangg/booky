package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ecleangg/booky/internal/accounting"
	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/support"
)

func (s *Service) handleDisputeCreated(ctx context.Context, evt Event) ([]domain.ObjectSnapshot, []domain.BalanceTransaction, []domain.AccountingFact, error) {
	var dispute Dispute
	if err := json.Unmarshal(evt.Data.Object, &dispute); err != nil {
		return nil, nil, nil, fmt.Errorf("decode dispute: %w", err)
	}
	postingDate := s.postingTime(dispute.Created)
	sourceCurrency := strings.ToUpper(dispute.Currency)
	settlementCurrency := sourceCurrency
	amountSEK := minorToOre(dispute.Amount)
	var balanceTransactions []domain.BalanceTransaction
	snapshots := []domain.ObjectSnapshot{{ObjectType: "dispute", ObjectID: dispute.ID, Livemode: evt.Livemode, Payload: evt.Data.Object}}
	if len(dispute.BalanceTransactions) > 0 && dispute.BalanceTransactions[0].ID != "" {
		btAPI, btRaw, err := s.Client.GetBalanceTransaction(ctx, dispute.BalanceTransactions[0].ID)
		if err != nil {
			return nil, nil, nil, err
		}
		bt := convertBalanceTransaction(s.Config, evt, btAPI, btRaw)
		balanceTransactions = append(balanceTransactions, bt)
		snapshots = append(snapshots, domain.ObjectSnapshot{ObjectType: "balance_transaction", ObjectID: bt.ID, Livemode: evt.Livemode, Payload: btRaw})
		settlementCurrency = strings.ToUpper(bt.Currency)
		amountSEK = settledGrossSEKOre(bt)
	}
	if amountSEK == 0 && sourceCurrency != "SEK" {
		obs, err := accounting.BuildReviewNoteFact(s.Config.Bokio.CompanyID, stripeAccountID(evt), "dispute:"+dispute.ID, "dispute", dispute.ID, "", evt.ID, postingDate, sourceCurrency, dispute.Amount, s.Config.Accounts.FallbackOBS, "dispute missing settled SEK amount", dispute)
		if err != nil {
			return nil, nil, nil, err
		}
		return snapshots, balanceTransactions, []domain.AccountingFact{obs}, nil
	}
	if len(balanceTransactions) == 0 && sourceCurrency != "SEK" {
		obs, err := accounting.BuildReviewNoteFact(s.Config.Bokio.CompanyID, stripeAccountID(evt), "dispute:"+dispute.ID, "dispute", dispute.ID, "", evt.ID, postingDate, sourceCurrency, dispute.Amount, s.Config.Accounts.FallbackOBS, "dispute missing settled SEK balance transaction", dispute)
		if err != nil {
			return nil, nil, nil, err
		}
		return snapshots, nil, []domain.AccountingFact{obs}, nil
	}
	stripeBalanceAccount, ok := s.Config.Accounts.StripeBalanceByCurrency[settlementCurrency]
	if !ok {
		reviewFacts, err := accounting.BuildReviewTransferFacts(accounting.ReviewTransferInput{
			BokioCompanyID:    s.Config.Bokio.CompanyID,
			StripeAccountID:   stripeAccountID(evt),
			SourceGroupID:     "dispute:" + dispute.ID,
			SourceObjectType:  "dispute",
			SourceObjectID:    dispute.ID,
			StripeEventID:     evt.ID,
			PostingDate:       postingDate,
			SourceCurrency:    sourceCurrency,
			SourceAmountMinor: dispute.Amount,
			AmountSEKOre:      amountSEK,
			DebitFactType:     "dispute_review_debit",
			CreditFactType:    "dispute_review_credit",
			DebitAccount:      s.Config.Accounts.Dispute,
			CreditAccount:     s.Config.Accounts.FallbackOBS,
			ReviewReason:      fmt.Sprintf("no Stripe balance account mapping for settlement currency %s", settlementCurrency),
			Payload:           dispute,
		})
		if err != nil {
			return nil, nil, nil, err
		}
		return snapshots, balanceTransactions, reviewFacts, nil
	}
	facts, err := accounting.BuildDisputeFacts(accounting.DisputeInput{
		BokioCompanyID:       s.Config.Bokio.CompanyID,
		StripeAccountID:      stripeAccountID(evt),
		SourceObjectID:       dispute.ID,
		SourceGroupID:        "dispute:" + dispute.ID,
		StripeEventID:        evt.ID,
		PostingDate:          postingDate,
		SourceCurrency:       sourceCurrency,
		AmountMinor:          dispute.Amount,
		AmountSEKOre:         amountSEK,
		DisputeAccount:       s.Config.Accounts.Dispute,
		StripeBalanceAccount: stripeBalanceAccount,
		Payload:              dispute,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return snapshots, balanceTransactions, facts, nil
}

func (s *Service) handlePayout(ctx context.Context, evt Event) ([]domain.ObjectSnapshot, []domain.BalanceTransaction, []domain.AccountingFact, error) {
	var eventPayout Payout
	if err := json.Unmarshal(evt.Data.Object, &eventPayout); err != nil {
		return nil, nil, nil, fmt.Errorf("decode payout from event: %w", err)
	}
	payout, payoutRaw, err := s.Client.GetPayout(ctx, eventPayout.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	balanceTxID := extractID(payout.BalanceTransaction)
	if balanceTxID == "" {
		return nil, nil, nil, fmt.Errorf("payout %s missing balance transaction", payout.ID)
	}
	btAPI, btRaw, err := s.Client.GetBalanceTransaction(ctx, balanceTxID)
	if err != nil {
		return nil, nil, nil, err
	}
	bt := convertBalanceTransaction(s.Config, evt, btAPI, btRaw)
	settlementCurrency := strings.ToUpper(bt.Currency)
	stripeBalanceAccount, ok := s.Config.Accounts.StripeBalanceByCurrency[settlementCurrency]
	if !ok {
		if settledGrossSEKOre(bt) == 0 && strings.ToUpper(payout.Currency) != "SEK" {
			obs, err := accounting.BuildReviewNoteFact(s.Config.Bokio.CompanyID, stripeAccountID(evt), "payout:"+payout.ID, "payout", payout.ID, bt.ID, evt.ID, s.postingTime(support.MaxInt64(payout.ArrivalDate, payout.Created)), strings.ToUpper(payout.Currency), support.AbsInt64(payout.Amount), s.Config.Accounts.FallbackOBS, "payout missing settled SEK amount", payout)
			if err != nil {
				return nil, nil, nil, err
			}
			return []domain.ObjectSnapshot{{ObjectType: "payout", ObjectID: payout.ID, Livemode: evt.Livemode, Payload: payoutRaw}, {ObjectType: "balance_transaction", ObjectID: bt.ID, Livemode: evt.Livemode, Payload: btRaw}}, []domain.BalanceTransaction{bt}, []domain.AccountingFact{obs}, nil
		}
		reviewFacts, err := accounting.BuildReviewTransferFacts(accounting.ReviewTransferInput{
			BokioCompanyID:             s.Config.Bokio.CompanyID,
			StripeAccountID:            stripeAccountID(evt),
			SourceGroupID:              "payout:" + payout.ID,
			SourceObjectType:           "payout",
			SourceObjectID:             payout.ID,
			StripeBalanceTransactionID: bt.ID,
			StripeEventID:              evt.ID,
			PostingDate:                s.postingTime(support.MaxInt64(payout.ArrivalDate, payout.Created)),
			SourceCurrency:             strings.ToUpper(payout.Currency),
			SourceAmountMinor:          support.AbsInt64(payout.Amount),
			AmountSEKOre:               settledGrossSEKOre(bt),
			DebitFactType:              "payout_review_bank",
			CreditFactType:             "payout_review_obs",
			DebitAccount:               s.Config.Accounts.Bank,
			CreditAccount:              s.Config.Accounts.FallbackOBS,
			ReviewReason:               fmt.Sprintf("no Stripe balance account mapping for settlement currency %s", settlementCurrency),
			Payload:                    payout,
		})
		if err != nil {
			return nil, nil, nil, err
		}
		return []domain.ObjectSnapshot{{ObjectType: "payout", ObjectID: payout.ID, Livemode: evt.Livemode, Payload: payoutRaw}, {ObjectType: "balance_transaction", ObjectID: bt.ID, Livemode: evt.Livemode, Payload: btRaw}}, []domain.BalanceTransaction{bt}, reviewFacts, nil
	}
	facts, err := accounting.BuildPayoutFacts(accounting.PayoutInput{
		BokioCompanyID:             s.Config.Bokio.CompanyID,
		StripeAccountID:            stripeAccountID(evt),
		SourceObjectID:             payout.ID,
		SourceGroupID:              "payout:" + payout.ID,
		StripeBalanceTransactionID: bt.ID,
		StripeEventID:              evt.ID,
		PostingDate:                s.postingTime(support.MaxInt64(payout.ArrivalDate, payout.Created)),
		SourceCurrency:             strings.ToUpper(payout.Currency),
		AmountMinor:                support.AbsInt64(payout.Amount),
		AmountSEKOre:               settledGrossSEKOre(bt),
		BankAccount:                s.Config.Accounts.Bank,
		StripeBalanceAccount:       stripeBalanceAccount,
		Payload:                    payout,
	})
	if settledGrossSEKOre(bt) == 0 && strings.ToUpper(payout.Currency) != "SEK" {
		obs, err := accounting.BuildReviewNoteFact(s.Config.Bokio.CompanyID, stripeAccountID(evt), "payout:"+payout.ID, "payout", payout.ID, bt.ID, evt.ID, s.postingTime(support.MaxInt64(payout.ArrivalDate, payout.Created)), strings.ToUpper(payout.Currency), support.AbsInt64(payout.Amount), s.Config.Accounts.FallbackOBS, "payout missing settled SEK amount", payout)
		if err != nil {
			return nil, nil, nil, err
		}
		return []domain.ObjectSnapshot{{ObjectType: "payout", ObjectID: payout.ID, Livemode: evt.Livemode, Payload: payoutRaw}, {ObjectType: "balance_transaction", ObjectID: bt.ID, Livemode: evt.Livemode, Payload: btRaw}}, []domain.BalanceTransaction{bt}, []domain.AccountingFact{obs}, nil
	}
	if err != nil {
		return nil, nil, nil, err
	}
	return []domain.ObjectSnapshot{{ObjectType: "payout", ObjectID: payout.ID, Livemode: evt.Livemode, Payload: payoutRaw}, {ObjectType: "balance_transaction", ObjectID: bt.ID, Livemode: evt.Livemode, Payload: btRaw}}, []domain.BalanceTransaction{bt}, facts, nil
}
