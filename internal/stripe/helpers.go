package stripe

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/support"
)

func convertBalanceTransaction(cfg config.Config, evt Event, bt BalanceTransactionAPI, raw json.RawMessage) domain.BalanceTransaction {
	amountSEK := amountToSEKOre(bt.Amount, domain.BalanceTransaction{Currency: strings.ToUpper(bt.Currency), AmountSEKOre: nil})
	feeSEK := amountToSEKOre(bt.Fee, domain.BalanceTransaction{Currency: strings.ToUpper(bt.Currency), FeeSEKOre: nil})
	netSEK := amountToSEKOre(bt.Net, domain.BalanceTransaction{Currency: strings.ToUpper(bt.Currency), NetSEKOre: nil})
	if strings.ToUpper(bt.Currency) != "SEK" && bt.ExchangeRate != nil {
		amountSEK = int64(math.Round(float64(bt.Amount) * *bt.ExchangeRate))
		feeSEK = int64(math.Round(float64(bt.Fee) * *bt.ExchangeRate))
		netSEK = int64(math.Round(float64(bt.Net) * *bt.ExchangeRate))
	}
	btDomain := domain.BalanceTransaction{
		ID:                bt.ID,
		StripeAccountID:   stripeAccountID(evt),
		SourceObjectType:  bt.Source.Object,
		SourceObjectID:    bt.Source.ID,
		Type:              bt.Type,
		ReportingCategory: bt.ReportingCategory,
		Status:            bt.Status,
		Currency:          strings.ToUpper(bt.Currency),
		CurrencyExponent:  currencyExponent(strings.ToUpper(bt.Currency)),
		AmountMinor:       bt.Amount,
		FeeMinor:          bt.Fee,
		NetMinor:          bt.Net,
		ExchangeRate:      bt.ExchangeRate,
		OccurredAt:        eventTimeInLocation(bt.Created, support.LocationOrUTC(cfg)),
		SourceEventID:     evt.ID,
		Payload:           raw,
	}
	btDomain.AmountSEKOre = &amountSEK
	btDomain.FeeSEKOre = &feeSEK
	btDomain.NetSEKOre = &netSEK
	if bt.AvailableOn > 0 {
		available := eventTimeInLocation(bt.AvailableOn, support.LocationOrUTC(cfg))
		btDomain.AvailableOn = &available
	}
	return btDomain
}

func eventTime(unixSeconds int64) time.Time {
	return time.Unix(unixSeconds, 0).UTC()
}

func eventTimeInLocation(unixSeconds int64, loc *time.Location) time.Time {
	return time.Unix(unixSeconds, 0).In(loc)
}

func (s *Service) postingTime(unixSeconds int64) time.Time {
	loc, err := s.Config.Location()
	if err != nil {
		return eventTime(unixSeconds)
	}
	return time.Unix(unixSeconds, 0).In(loc)
}

func stripeAccountID(evt Event) string {
	if evt.Account != "" {
		return evt.Account
	}
	return "self"
}

func extractID(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case StripeRef:
		return v.ID
	case *StripeRef:
		if v != nil {
			return v.ID
		}
	case map[string]any:
		if id, ok := v["id"].(string); ok {
			return id
		}
	}
	return ""
}

func currencyExponent(currency string) int16 {
	switch strings.ToUpper(currency) {
	case "JPY":
		return 0
	default:
		return 2
	}
}

func amountToSEKOre(amountMinor int64, bt domain.BalanceTransaction) int64 {
	if strings.ToUpper(bt.Currency) == "SEK" || bt.Currency == "" {
		return amountMinor
	}
	if bt.AmountSEKOre != nil && bt.AmountMinor != 0 && amountMinor == bt.AmountMinor {
		return *bt.AmountSEKOre
	}
	if bt.FeeSEKOre != nil && bt.FeeMinor != 0 && amountMinor == bt.FeeMinor {
		return *bt.FeeSEKOre
	}
	if bt.NetSEKOre != nil && bt.NetMinor != 0 && amountMinor == bt.NetMinor {
		return *bt.NetSEKOre
	}
	if bt.ExchangeRate != nil {
		return int64(math.Round(float64(amountMinor) * *bt.ExchangeRate))
	}
	return 0
}

func settledGrossSEKOre(bt domain.BalanceTransaction) int64 {
	if bt.AmountSEKOre != nil {
		return support.AbsInt64(*bt.AmountSEKOre)
	}
	if strings.ToUpper(bt.Currency) == "SEK" {
		return support.AbsInt64(bt.AmountMinor)
	}
	return support.AbsInt64(amountToSEKOre(bt.AmountMinor, bt))
}

func settledFeeSEKOre(bt domain.BalanceTransaction) int64 {
	if bt.FeeSEKOre != nil {
		return support.AbsInt64(*bt.FeeSEKOre)
	}
	if strings.ToUpper(bt.Currency) == "SEK" {
		return support.AbsInt64(bt.FeeMinor)
	}
	return support.AbsInt64(amountToSEKOre(bt.FeeMinor, bt))
}

func (s *Service) getChargeReadyForAccounting(ctx context.Context, chargeID string) (Charge, json.RawMessage, error) {
	var lastCharge Charge
	var lastRaw json.RawMessage
	var err error

	for attempt := 0; attempt < 5; attempt++ {
		lastCharge, lastRaw, err = s.Client.GetCharge(ctx, chargeID)
		if err != nil {
			return Charge{}, nil, err
		}
		if extractID(lastCharge.BalanceTransaction) != "" {
			return lastCharge, lastRaw, nil
		}
		if attempt == 4 {
			break
		}
		select {
		case <-ctx.Done():
			return Charge{}, nil, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 250 * time.Millisecond):
		}
	}

	return lastCharge, lastRaw, nil
}

func valueOrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func proportionalAmount(total int64, partMinor int64, wholeMinor int64) int64 {
	if wholeMinor == 0 {
		return 0
	}
	return int64(math.Round(float64(total) * float64(partMinor) / float64(wholeMinor)))
}

func minorToOre(amountMinor int64) int64 {
	return amountMinor
}
