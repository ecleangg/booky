package accounting

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/support"
	"github.com/google/uuid"
)

func BuildSaleFacts(input SaleInput) ([]domain.AccountingFact, error) {
	if input.GrossSEKOre <= 0 {
		return nil, fmt.Errorf("gross amount must be positive")
	}
	payload, err := json.Marshal(input.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal sale payload: %w", err)
	}

	facts := []domain.AccountingFact{
		newFact(input.BokioCompanyID, input.StripeAccountID, input.TaxCaseID, input.SourceGroupPrefix+":sale", input.SourceObjectType, input.SourceObjectID, input.StripeBalanceTransactionID, input.StripeEventID, "sale_receivable", input.ChargeDate, input.MarketCode, input.VATTreatment, input.SourceCurrency, input.GrossMinor, input.GrossSEKOre, input.ReceivableAccount, domain.DirectionDebit, domain.FactStatusPending, nil, payload),
		newFact(input.BokioCompanyID, input.StripeAccountID, input.TaxCaseID, input.SourceGroupPrefix+":sale", input.SourceObjectType, input.SourceObjectID, input.StripeBalanceTransactionID, input.StripeEventID, "sale_revenue", input.ChargeDate, input.MarketCode, input.VATTreatment, input.SourceCurrency, input.GrossMinor, input.RevenueSEKOre, input.RevenueAccount, domain.DirectionCredit, domain.FactStatusPending, nil, payload),
	}

	if input.VATSEKOre > 0 && input.OutputVATAccount != 0 {
		facts = append(facts, newFact(input.BokioCompanyID, input.StripeAccountID, input.TaxCaseID, input.SourceGroupPrefix+":sale", input.SourceObjectType, input.SourceObjectID, input.StripeBalanceTransactionID, input.StripeEventID, "sale_output_vat", input.ChargeDate, input.MarketCode, input.VATTreatment, input.SourceCurrency, input.GrossMinor, input.VATSEKOre, input.OutputVATAccount, domain.DirectionCredit, domain.FactStatusPending, nil, payload))
	}

	if input.FeeSEKOre > 0 {
		feeCurrency := input.FeeCurrency
		if feeCurrency == "" {
			feeCurrency = input.SourceCurrency
		}
		feeMinor := input.FeeMinor
		if feeMinor == 0 {
			feeMinor = input.GrossMinor
		}
		facts = append(facts,
			newFact(input.BokioCompanyID, input.StripeAccountID, input.TaxCaseID, input.SourceGroupPrefix+":fee", input.SourceObjectType, input.SourceObjectID, input.StripeBalanceTransactionID, input.StripeEventID, "stripe_fee_expense", input.ChargeDate, input.MarketCode, input.VATTreatment, feeCurrency, feeMinor, input.FeeSEKOre, input.FeeExpenseAccount, domain.DirectionDebit, domain.FactStatusPending, nil, payload),
			newFact(input.BokioCompanyID, input.StripeAccountID, input.TaxCaseID, input.SourceGroupPrefix+":fee", input.SourceObjectType, input.SourceObjectID, input.StripeBalanceTransactionID, input.StripeEventID, "stripe_fee_balance", input.ChargeDate, input.MarketCode, input.VATTreatment, feeCurrency, feeMinor, input.FeeSEKOre, input.StripeBalanceAccount, domain.DirectionCredit, domain.FactStatusPending, nil, payload),
		)
		if input.FeeReverseVATSEKOre > 0 {
			facts = append(facts,
				newFact(input.BokioCompanyID, input.StripeAccountID, input.TaxCaseID, input.SourceGroupPrefix+":fee", input.SourceObjectType, input.SourceObjectID, input.StripeBalanceTransactionID, input.StripeEventID, "stripe_fee_input_vat", input.ChargeDate, input.MarketCode, input.VATTreatment, feeCurrency, feeMinor, input.FeeReverseVATSEKOre, input.FeeInputVATAccount, domain.DirectionDebit, domain.FactStatusPending, nil, payload),
				newFact(input.BokioCompanyID, input.StripeAccountID, input.TaxCaseID, input.SourceGroupPrefix+":fee", input.SourceObjectType, input.SourceObjectID, input.StripeBalanceTransactionID, input.StripeEventID, "stripe_fee_output_vat", input.ChargeDate, input.MarketCode, input.VATTreatment, feeCurrency, feeMinor, input.FeeReverseVATSEKOre, input.FeeOutputVATAccount, domain.DirectionCredit, domain.FactStatusPending, nil, payload),
			)
		}
	}

	if input.AvailableOn != nil {
		facts = append(facts,
			newFact(input.BokioCompanyID, input.StripeAccountID, input.TaxCaseID, input.SourceGroupPrefix+":available", input.SourceObjectType, input.SourceObjectID, input.StripeBalanceTransactionID, input.StripeEventID, "stripe_available_debit", *input.AvailableOn, input.MarketCode, input.VATTreatment, input.SourceCurrency, input.GrossMinor, input.GrossSEKOre, input.StripeBalanceAccount, domain.DirectionDebit, domain.FactStatusPending, nil, payload),
			newFact(input.BokioCompanyID, input.StripeAccountID, input.TaxCaseID, input.SourceGroupPrefix+":available", input.SourceObjectType, input.SourceObjectID, input.StripeBalanceTransactionID, input.StripeEventID, "stripe_available_credit", *input.AvailableOn, input.MarketCode, input.VATTreatment, input.SourceCurrency, input.GrossMinor, input.GrossSEKOre, input.ReceivableAccount, domain.DirectionCredit, domain.FactStatusPending, nil, payload),
		)
	}

	return facts, nil
}

func BuildRefundFacts(input RefundInput) ([]domain.AccountingFact, error) {
	payload, err := json.Marshal(input.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal refund payload: %w", err)
	}

	facts := []domain.AccountingFact{
		newFact(input.BokioCompanyID, input.StripeAccountID, input.TaxCaseID, input.SourceGroupID, "refund", input.SourceObjectID, input.StripeBalanceTransactionID, input.StripeEventID, "refund_receivable", input.PostingDate, input.MarketCode, input.VATTreatment, input.SourceCurrency, input.GrossMinor, input.GrossSEKOre, input.ReceivableAccount, domain.DirectionCredit, domain.FactStatusPending, nil, payload),
		newFact(input.BokioCompanyID, input.StripeAccountID, input.TaxCaseID, input.SourceGroupID, "refund", input.SourceObjectID, input.StripeBalanceTransactionID, input.StripeEventID, "refund_revenue", input.PostingDate, input.MarketCode, input.VATTreatment, input.SourceCurrency, input.GrossMinor, input.RevenueSEKOre, input.RevenueAccount, domain.DirectionDebit, domain.FactStatusPending, nil, payload),
	}
	if input.VATSEKOre > 0 && input.OutputVATAccount != 0 {
		facts = append(facts, newFact(input.BokioCompanyID, input.StripeAccountID, input.TaxCaseID, input.SourceGroupID, "refund", input.SourceObjectID, input.StripeBalanceTransactionID, input.StripeEventID, "refund_output_vat", input.PostingDate, input.MarketCode, input.VATTreatment, input.SourceCurrency, input.GrossMinor, input.VATSEKOre, input.OutputVATAccount, domain.DirectionDebit, domain.FactStatusPending, nil, payload))
	}
	return facts, nil
}

func BuildPayoutFacts(input PayoutInput) ([]domain.AccountingFact, error) {
	payload, err := json.Marshal(input.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payout payload: %w", err)
	}

	return []domain.AccountingFact{
		newFact(input.BokioCompanyID, input.StripeAccountID, input.TaxCaseID, input.SourceGroupID, "payout", input.SourceObjectID, input.StripeBalanceTransactionID, input.StripeEventID, "payout_bank", input.PostingDate, "", "", input.SourceCurrency, input.AmountMinor, input.AmountSEKOre, input.BankAccount, domain.DirectionDebit, domain.FactStatusPending, nil, payload),
		newFact(input.BokioCompanyID, input.StripeAccountID, input.TaxCaseID, input.SourceGroupID, "payout", input.SourceObjectID, input.StripeBalanceTransactionID, input.StripeEventID, "payout_stripe_balance", input.PostingDate, "", "", input.SourceCurrency, input.AmountMinor, input.AmountSEKOre, input.StripeBalanceAccount, domain.DirectionCredit, domain.FactStatusPending, nil, payload),
	}, nil
}

func BuildDisputeFacts(input DisputeInput) ([]domain.AccountingFact, error) {
	payload, err := json.Marshal(input.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal dispute payload: %w", err)
	}

	return []domain.AccountingFact{
		newFact(input.BokioCompanyID, input.StripeAccountID, input.TaxCaseID, input.SourceGroupID, "dispute", input.SourceObjectID, input.StripeBalanceTransactionID, input.StripeEventID, "dispute_debit", input.PostingDate, "", "", input.SourceCurrency, input.AmountMinor, input.AmountSEKOre, input.DisputeAccount, domain.DirectionDebit, domain.FactStatusPending, nil, payload),
		newFact(input.BokioCompanyID, input.StripeAccountID, input.TaxCaseID, input.SourceGroupID, "dispute", input.SourceObjectID, input.StripeBalanceTransactionID, input.StripeEventID, "dispute_credit", input.PostingDate, "", "", input.SourceCurrency, input.AmountMinor, input.AmountSEKOre, input.StripeBalanceAccount, domain.DirectionCredit, domain.FactStatusPending, nil, payload),
	}, nil
}

func BuildReviewTransferFacts(input ReviewTransferInput) ([]domain.AccountingFact, error) {
	if input.AmountSEKOre <= 0 {
		return nil, fmt.Errorf("review transfer amount must be positive")
	}
	b, err := json.Marshal(input.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal review payload: %w", err)
	}
	reason := input.ReviewReason
	return []domain.AccountingFact{
		newFact(input.BokioCompanyID, input.StripeAccountID, input.TaxCaseID, input.SourceGroupID, input.SourceObjectType, input.SourceObjectID, input.StripeBalanceTransactionID, input.StripeEventID, input.DebitFactType, input.PostingDate, input.MarketCode, input.VATTreatment, input.SourceCurrency, input.SourceAmountMinor, input.AmountSEKOre, input.DebitAccount, domain.DirectionDebit, domain.FactStatusNeedsReview, &reason, b),
		newFact(input.BokioCompanyID, input.StripeAccountID, input.TaxCaseID, input.SourceGroupID, input.SourceObjectType, input.SourceObjectID, input.StripeBalanceTransactionID, input.StripeEventID, input.CreditFactType, input.PostingDate, input.MarketCode, input.VATTreatment, input.SourceCurrency, input.SourceAmountMinor, input.AmountSEKOre, input.CreditAccount, domain.DirectionCredit, domain.FactStatusNeedsReview, &reason, b),
	}, nil
}

func BuildReviewNoteFact(companyID uuid.UUID, stripeAccountID, sourceGroupID, sourceObjectType, sourceObjectID, stripeBalanceTxID, stripeEventID string, postingDate time.Time, currency string, amountMinor int64, obsAccount int, reason string, payload any) (domain.AccountingFact, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return domain.AccountingFact{}, fmt.Errorf("marshal review note payload: %w", err)
	}
	return newFact(companyID, stripeAccountID, nil, sourceGroupID, sourceObjectType, sourceObjectID, stripeBalanceTxID, stripeEventID, "unknown_obs", postingDate, "", "review", currency, amountMinor, 0, obsAccount, domain.DirectionDebit, domain.FactStatusNeedsReview, &reason, b), nil
}

func newFact(companyID uuid.UUID, stripeAccountID string, taxCaseID *uuid.UUID, sourceGroupID, sourceObjectType, sourceObjectID, stripeBalanceTxID, stripeEventID, factType string, postingDate time.Time, marketCode, vatTreatment, sourceCurrency string, sourceAmountMinor, amountSEKOre int64, bokioAccount int, direction, status string, reviewReason *string, payload []byte) domain.AccountingFact {
	var balanceIDPtr, eventIDPtr, marketPtr, vatPtr, currencyPtr *string
	var sourceAmountPtr *int64
	if stripeBalanceTxID != "" {
		balanceIDPtr = &stripeBalanceTxID
	}
	if stripeEventID != "" {
		eventIDPtr = &stripeEventID
	}
	if marketCode != "" {
		marketPtr = &marketCode
	}
	if vatTreatment != "" {
		vatPtr = &vatTreatment
	}
	if sourceCurrency != "" {
		currencyPtr = &sourceCurrency
	}
	if sourceAmountMinor != 0 || sourceCurrency != "" {
		sourceAmountPtr = &sourceAmountMinor
	}

	return domain.AccountingFact{
		ID:                         uuid.New(),
		BokioCompanyID:             companyID,
		StripeAccountID:            stripeAccountID,
		TaxCaseID:                  taxCaseID,
		SourceGroupID:              sourceGroupID,
		SourceObjectType:           sourceObjectType,
		SourceObjectID:             sourceObjectID,
		StripeBalanceTransactionID: balanceIDPtr,
		StripeEventID:              eventIDPtr,
		FactType:                   factType,
		PostingDate:                dateOnly(postingDate),
		MarketCode:                 marketPtr,
		VATTreatment:               vatPtr,
		SourceCurrency:             currencyPtr,
		SourceAmountMinor:          sourceAmountPtr,
		AmountSEKOre:               support.AbsInt64(amountSEKOre),
		BokioAccount:               bokioAccount,
		Direction:                  direction,
		Status:                     status,
		ReviewReason:               reviewReason,
		Payload:                    payload,
	}
}
