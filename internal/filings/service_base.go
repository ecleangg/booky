package filings

import (
	"context"
	"log/slog"
	"time"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/integrations"
	"github.com/ecleangg/booky/internal/notify"
	"github.com/ecleangg/booky/internal/store"
	"github.com/ecleangg/booky/internal/support"
)

type Service struct {
	Config config.Config
	Repo   *store.Repository
	Notify notify.Notifier
	Tenants *integrations.Service
	Logger *slog.Logger
}

type PeriodStatus struct {
	Period        domain.FilingPeriod  `json:"period"`
	ReadyEntries  int                  `json:"ready_entries"`
	ReviewEntries int                  `json:"review_entries"`
	LatestExport  *domain.FilingExport `json:"latest_export,omitempty"`
}

func NewService(cfg config.Config, repo *store.Repository, notifier notify.Notifier, tenants *integrations.Service, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		Config: cfg,
		Repo:   repo,
		Notify: notifier,
		Tenants: tenants,
		Logger: logger,
	}
}

func (s *Service) Enabled() bool {
	return s != nil && s.Config.Filings.Enabled
}

func (s *Service) BuildWebhookEntries(ctx context.Context, taxCases []domain.TaxCase, facts []domain.AccountingFact) ([]domain.OSSUnionEntry, []domain.PeriodicSummaryEntry, []domain.FilingPeriod, error) {
	return s.buildEntries(ctx, facts, newMemorySource(taxCases))
}

func locationOrUTC(cfg config.Config) *time.Location {
	return support.LocationOrUTC(cfg)
}
