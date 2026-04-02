package accounting

import (
	"log/slog"

	"github.com/ecleangg/booky/internal/bokio"
	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/integrations"
	"github.com/ecleangg/booky/internal/notify"
	"github.com/ecleangg/booky/internal/pdf"
	"github.com/ecleangg/booky/internal/store"
)

type Service struct {
	Config config.Config
	Repo   *store.Repository
	Bokio  *bokio.Client
	Notify notify.Notifier
	PDF    *pdf.Generator
	Tenants *integrations.Service
	Logger *slog.Logger
}

func NewService(cfg config.Config, repo *store.Repository, bokioClient *bokio.Client, notifier notify.Notifier, pdfGenerator *pdf.Generator, tenants *integrations.Service, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{Config: cfg, Repo: repo, Bokio: bokioClient, Notify: notifier, PDF: pdfGenerator, Tenants: tenants, Logger: logger}
}
