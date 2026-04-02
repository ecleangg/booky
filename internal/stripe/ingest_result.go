package stripe

import "github.com/ecleangg/booky/internal/domain"

type ingestResult struct {
	Snapshots      []domain.ObjectSnapshot
	BalanceTxs     []domain.BalanceTransaction
	TaxCases       []domain.TaxCase
	TaxCaseObjects []domain.TaxCaseObject
	Facts          []domain.AccountingFact
}

func mergeIngest(parts ...ingestResult) ingestResult {
	var out ingestResult
	for _, part := range parts {
		out.Snapshots = append(out.Snapshots, part.Snapshots...)
		out.BalanceTxs = append(out.BalanceTxs, part.BalanceTxs...)
		out.TaxCases = append(out.TaxCases, part.TaxCases...)
		out.TaxCaseObjects = append(out.TaxCaseObjects, part.TaxCaseObjects...)
		out.Facts = append(out.Facts, part.Facts...)
	}
	return out
}
