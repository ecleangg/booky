package stripe

import (
	"context"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/store"
	"github.com/ecleangg/booky/internal/tax"
	"github.com/google/uuid"
)

func (s *Service) GetTaxCase(ctx context.Context, id uuid.UUID) (tax.ExpandedCase, error) {
	return s.Tax.GetCase(ctx, id)
}

func (s *Service) RebuildTaxCase(ctx context.Context, id uuid.UUID) (tax.ExpandedCase, error) {
	current, err := s.Repo.Queries().GetTaxCase(ctx, id)
	if err != nil {
		return tax.ExpandedCase{}, err
	}
	rebuilt, objects, snapshots, err := s.rebuildStoredTaxCase(ctx, current)
	if err != nil {
		return tax.ExpandedCase{}, err
	}
	rebuilt.ID = current.ID
	for i := range objects {
		objects[i].TaxCaseID = current.ID
	}
	if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
		for _, snapshot := range snapshots {
			if err := q.UpsertObjectSnapshot(ctx, snapshot); err != nil {
				return err
			}
		}
		persistedID, err := q.UpsertTaxCase(ctx, rebuilt)
		if err != nil {
			return err
		}
		return q.ReplaceTaxCaseObjects(ctx, persistedID, objects)
	}); err != nil {
		return tax.ExpandedCase{}, err
	}
	if s.Filings != nil && s.Filings.Enabled() {
		facts, err := s.Repo.Queries().ListFactsByTaxCaseIDs(ctx, []uuid.UUID{id})
		if err != nil {
			return tax.ExpandedCase{}, err
		}
		if err := s.Filings.SyncWebhookEntries(ctx, []domain.TaxCase{rebuilt}, facts); err != nil {
			return tax.ExpandedCase{}, err
		}
	}
	return s.Tax.GetCase(ctx, id)
}

func (s *Service) RecordManualTaxEvidence(ctx context.Context, evidence domain.ManualTaxEvidence) (tax.ExpandedCase, error) {
	if _, err := s.Tax.RecordManualEvidence(ctx, evidence); err != nil {
		return tax.ExpandedCase{}, err
	}
	return s.RebuildTaxCase(ctx, evidence.TaxCaseID)
}
