package filings

import (
	"context"
	"log/slog"
	"time"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/notify"
	"github.com/ecleangg/booky/internal/store"
	"github.com/ecleangg/booky/internal/support"
)

type RateClient interface {
	OSSPeriodEndEURSEK(ctx context.Context, period string) (domain.FXRate, error)
	PSMonthlyAverage(ctx context.Context, currency, period string) (domain.FXRate, error)
}

type Service struct {
	Config config.Config
	Repo   *store.Repository
	Notify notify.Notifier
	Rates  RateClient
	Logger *slog.Logger
}

type PeriodStatus struct {
	Period        domain.FilingPeriod  `json:"period"`
	ReadyEntries  int                  `json:"ready_entries"`
	ReviewEntries int                  `json:"review_entries"`
	LatestExport  *domain.FilingExport `json:"latest_export,omitempty"`
}

func NewService(cfg config.Config, repo *store.Repository, notifier notify.Notifier, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		Config: cfg,
		Repo:   repo,
		Notify: notifier,
		Rates:  NewHTTPRateClient(cfg.Filings.FX),
		Logger: logger,
	}
}

func (s *Service) Enabled() bool {
	return s != nil && s.Config.Filings.Enabled
}

func (s *Service) BuildWebhookEntries(ctx context.Context, snapshots []domain.ObjectSnapshot, facts []domain.AccountingFact) ([]domain.OSSUnionEntry, []domain.PeriodicSummaryEntry, []domain.FilingPeriod, error) {
	return s.buildEntries(ctx, facts, newMemorySource(snapshots, nil))
}

func locationOrUTC(cfg config.Config) *time.Location {
	return support.LocationOrUTC(cfg)
}
