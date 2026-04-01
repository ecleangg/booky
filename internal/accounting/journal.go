package accounting

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/support"
	"github.com/google/uuid"
)

func AggregateJournalItems(facts []domain.AccountingFact, roundingAccount int, toleranceOre int64) ([]domain.JournalItem, *domain.AccountingFact, map[string]any, error) {
	totals := map[string]int64{}
	var debitTotal, creditTotal int64

	for _, fact := range facts {
		key := fmt.Sprintf("%d:%s", fact.BokioAccount, fact.Direction)
		totals[key] += fact.AmountSEKOre
		if fact.Direction == domain.DirectionDebit {
			debitTotal += fact.AmountSEKOre
		} else {
			creditTotal += fact.AmountSEKOre
		}
	}

	var roundingFact *domain.AccountingFact
	diff := debitTotal - creditTotal
	if diff != 0 {
		if support.AbsInt64(diff) > toleranceOre {
			return nil, nil, nil, fmt.Errorf("unbalanced journal: debit=%d credit=%d diff=%d exceeds tolerance=%d", debitTotal, creditTotal, diff, toleranceOre)
		}
		rounding := domain.AccountingFact{
			ID:            uuid.New(),
			SourceGroupID: "rounding",
			FactType:      "rounding",
			PostingDate:   facts[0].PostingDate,
			AmountSEKOre:  support.AbsInt64(diff),
			BokioAccount:  roundingAccount,
			Status:        domain.FactStatusPending,
		}
		if diff > 0 {
			rounding.Direction = domain.DirectionCredit
			creditTotal += rounding.AmountSEKOre
		} else {
			rounding.Direction = domain.DirectionDebit
			debitTotal += rounding.AmountSEKOre
		}
		roundingFact = &rounding
		totals[fmt.Sprintf("%d:%s", rounding.BokioAccount, rounding.Direction)] += rounding.AmountSEKOre
	}

	keys := make([]string, 0, len(totals))
	for key := range totals {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	items := make([]domain.JournalItem, 0, len(keys))
	for _, key := range keys {
		parts := strings.Split(key, ":")
		account := 0
		_, _ = fmt.Sscanf(parts[0], "%d", &account)
		amount := float64(totals[key]) / 100.0
		item := domain.JournalItem{Account: account}
		if len(parts) > 1 && parts[1] == domain.DirectionDebit {
			item.Debit = amount
		} else {
			item.Credit = amount
		}
		items = append(items, item)
	}

	summary := map[string]any{
		"fact_count":          len(facts),
		"debit_total_ore":     debitTotal,
		"credit_total_ore":    creditTotal,
		"rounding_added":      roundingFact != nil,
		"rounding_amount_ore": 0,
	}
	if roundingFact != nil {
		summary["rounding_amount_ore"] = roundingFact.AmountSEKOre
	}

	return items, roundingFact, summary, nil
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func directionForSignedAmount(amount int64) string {
	if amount < 0 {
		return domain.DirectionCredit
	}
	return domain.DirectionDebit
}
