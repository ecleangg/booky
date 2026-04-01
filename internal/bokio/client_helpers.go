package bokio

import (
	"fmt"
	"sort"

	"github.com/ecleangg/booky/internal/domain"
)

func normalizeJournalItems(items []domain.JournalItem) []string {
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		normalized = append(normalized, fmt.Sprintf("%d|%.2f|%.2f", item.Account, item.Debit, item.Credit))
	}
	sort.Strings(normalized)
	return normalized
}

func normalizeJournalEntryItems(items []JournalEntryItem) []string {
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		normalized = append(normalized, fmt.Sprintf("%d|%.2f|%.2f", item.Account, item.Debit, item.Credit))
	}
	sort.Strings(normalized)
	return normalized
}

func sameJournalItems(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
