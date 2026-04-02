package filings

import (
	"context"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/store"
	"github.com/google/uuid"
)

type entrySource interface {
	TaxCase(ctx context.Context, id uuid.UUID) (domain.TaxCase, error)
}

type memorySource struct {
	taxCases map[uuid.UUID]domain.TaxCase
}

func newMemorySource(taxCases []domain.TaxCase) entrySource {
	source := &memorySource{taxCases: make(map[uuid.UUID]domain.TaxCase, len(taxCases))}
	for _, taxCase := range taxCases {
		source.taxCases[taxCase.ID] = taxCase
	}
	return source
}

func (s *memorySource) TaxCase(_ context.Context, id uuid.UUID) (domain.TaxCase, error) {
	out, ok := s.taxCases[id]
	if !ok {
		return domain.TaxCase{}, store.ErrNotFound
	}
	return out, nil
}

type repositorySource struct {
	repo *store.Repository
}

func newRepositorySource(repo *store.Repository) entrySource {
	return &repositorySource{repo: repo}
}

func (s *repositorySource) TaxCase(ctx context.Context, id uuid.UUID) (domain.TaxCase, error) {
	return s.repo.Queries().GetTaxCase(ctx, id)
}
