package store

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/ecleangg/booky/internal/domain"
)

func scanAccountingFact(row interface{ Scan(dest ...any) error }) (domain.AccountingFact, error) {
	var fact domain.AccountingFact
	var stripeBalanceID, stripeEventID sql.NullString
	var marketCode, vatTreatment, sourceCurrency, reviewReason sql.NullString
	var sourceAmount sql.NullInt64
	var payload []byte
	if err := row.Scan(
		&fact.ID,
		&fact.BokioCompanyID,
		&fact.StripeAccountID,
		&fact.SourceGroupID,
		&fact.SourceObjectType,
		&fact.SourceObjectID,
		&stripeBalanceID,
		&stripeEventID,
		&fact.FactType,
		&fact.PostingDate,
		&marketCode,
		&vatTreatment,
		&sourceCurrency,
		&sourceAmount,
		&fact.AmountSEKOre,
		&fact.BokioAccount,
		&fact.Direction,
		&fact.Status,
		&reviewReason,
		&payload,
		&fact.CreatedAt,
		&fact.UpdatedAt,
	); err != nil {
		return domain.AccountingFact{}, fmt.Errorf("scan accounting fact: %w", err)
	}
	if stripeBalanceID.Valid {
		fact.StripeBalanceTransactionID = &stripeBalanceID.String
	}
	if stripeEventID.Valid {
		fact.StripeEventID = &stripeEventID.String
	}
	if marketCode.Valid {
		fact.MarketCode = &marketCode.String
	}
	if vatTreatment.Valid {
		fact.VATTreatment = &vatTreatment.String
	}
	if sourceCurrency.Valid {
		fact.SourceCurrency = &sourceCurrency.String
	}
	if sourceAmount.Valid {
		fact.SourceAmountMinor = &sourceAmount.Int64
	}
	if reviewReason.Valid {
		fact.ReviewReason = &reviewReason.String
	}
	fact.Payload = payload
	return fact, nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableJSON(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return []byte(value)
}

func ChecksumJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha1.Sum(b)
	return hex.EncodeToString(sum[:]), nil
}
