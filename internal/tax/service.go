package tax

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/store"
	"github.com/google/uuid"
)

type Service struct {
	Config config.Config
	Repo   *store.Repository
	Logger *slog.Logger
}

type ExpandedCase struct {
	Case           domain.TaxCase             `json:"case"`
	Objects        []domain.TaxCaseObject     `json:"objects"`
	ManualEvidence []domain.ManualTaxEvidence `json:"manual_evidence"`
}

func NewService(cfg config.Config, repo *store.Repository, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{Config: cfg, Repo: repo, Logger: logger}
}

func (s *Service) BuildCaseFromSnapshots(ctx context.Context, livemode bool, snapshots []domain.ObjectSnapshot) (domain.TaxCase, []domain.TaxCaseObject, error) {
	preview, err := BuildCase(s.Config.Bokio.CompanyID, livemode, snapshots, nil, nil)
	if err != nil {
		return domain.TaxCase{}, nil, err
	}

	var manual []domain.ManualTaxEvidence
	var existingID *uuid.UUID
	existing, err := s.Repo.Queries().GetTaxCaseByRoot(ctx, s.Config.Bokio.CompanyID, preview.Case.RootObjectType, preview.Case.RootObjectID)
	if err == nil {
		existingID = &existing.ID
		manual, err = s.Repo.Queries().ListManualTaxEvidence(ctx, existing.ID)
		if err != nil {
			return domain.TaxCase{}, nil, err
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		return domain.TaxCase{}, nil, err
	}

	result, err := BuildCase(s.Config.Bokio.CompanyID, livemode, snapshots, manual, existingID)
	if err != nil {
		return domain.TaxCase{}, nil, err
	}
	return result.Case, result.Objects, nil
}

func (s *Service) GetCase(ctx context.Context, id uuid.UUID) (ExpandedCase, error) {
	taxCase, err := s.Repo.Queries().GetTaxCase(ctx, id)
	if err != nil {
		return ExpandedCase{}, err
	}
	objects, err := s.Repo.Queries().ListTaxCaseObjects(ctx, id)
	if err != nil {
		return ExpandedCase{}, err
	}
	manual, err := s.Repo.Queries().ListManualTaxEvidence(ctx, id)
	if err != nil {
		return ExpandedCase{}, err
	}
	return ExpandedCase{Case: taxCase, Objects: objects, ManualEvidence: manual}, nil
}

func (s *Service) RecordManualEvidence(ctx context.Context, evidence domain.ManualTaxEvidence) (ExpandedCase, error) {
	if evidence.ID == uuid.Nil {
		evidence.ID = uuid.New()
	}
	if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
		return q.UpsertManualTaxEvidence(ctx, evidence)
	}); err != nil {
		return ExpandedCase{}, err
	}
	return s.GetCase(ctx, evidence.TaxCaseID)
}
