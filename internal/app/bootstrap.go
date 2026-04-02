package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ecleangg/booky/internal/accounting"
	"github.com/ecleangg/booky/internal/bokio"
	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/filings"
	"github.com/ecleangg/booky/internal/httpapi"
	"github.com/ecleangg/booky/internal/integrations"
	"github.com/ecleangg/booky/internal/jobs"
	"github.com/ecleangg/booky/internal/notify"
	"github.com/ecleangg/booky/internal/pdf"
	"github.com/ecleangg/booky/internal/store"
	"github.com/ecleangg/booky/internal/stripe"
	"github.com/ecleangg/booky/internal/tax"
)

type App struct {
	Config     config.Config
	Logger     *slog.Logger
	Repo       *store.Repository
	HTTPServer *http.Server
	Scheduler  *jobs.Scheduler
	Filings    *filings.Service
}

func New(ctx context.Context, configPath string) (*App, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	repo, err := store.New(ctx, cfg.Postgres.DSN)
	if err != nil {
		return nil, err
	}
	if err := repo.Ping(ctx); err != nil {
		repo.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	migrationsDir := filepath.Join(filepath.Dir(configPath), "..", "migrations")
	if _, err := os.Stat(migrationsDir); err != nil {
		migrationsDir = "migrations"
	}
	if err := repo.RunMigrations(ctx, migrationsDir); err != nil {
		repo.Close()
		return nil, err
	}

	stripeClient := stripe.NewClient(cfg.Stripe)
	bokioClient := bokio.NewClient(cfg.Bokio)
	integrationService, err := integrations.NewService(cfg, repo, logger)
	if err != nil {
		repo.Close()
		return nil, err
	}
	var notifier notify.Notifier
	if cfg.Notifications.Resend.Enabled {
		notifier = notify.NewResendNotifier(cfg.Notifications.Resend)
	}
	pdfGenerator := pdf.NewGenerator()
	taxService := tax.NewService(cfg, repo, logger)
	accountingService := accounting.NewService(cfg, repo, bokioClient, notifier, pdfGenerator, integrationService, logger)
	filingsService := filings.NewService(cfg, repo, notifier, integrationService, logger)
	stripeService := stripe.NewService(cfg, repo, stripeClient, taxService, notifier, filingsService, integrationService, logger)
	router := httpapi.NewRouter(cfg, stripeService, accountingService, filingsService, bokioClient, integrationService, logger)

	server := &http.Server{
		Addr:         cfg.HTTP.ListenAddr,
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout(),
		WriteTimeout: cfg.WriteTimeout(),
		IdleTimeout:  cfg.IdleTimeout(),
	}

	return &App{
		Config:     cfg,
		Logger:     logger,
		Repo:       repo,
		HTTPServer: server,
		Scheduler:  jobs.NewScheduler(cfg, accountingService, filingsService, integrationService, logger),
		Filings:    filingsService,
	}, nil
}
