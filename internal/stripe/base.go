package stripe

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/filings"
	"github.com/ecleangg/booky/internal/notify"
	"github.com/ecleangg/booky/internal/store"
)

type Client struct {
	apiKey        string
	webhookSecret string
	baseURL       string
	httpClient    *http.Client
}

type Service struct {
	Config  config.Config
	Repo    *store.Repository
	Client  *Client
	Notify  notify.Notifier
	Filings *filings.Service
	Logger  *slog.Logger
}

func NewClient(cfg config.StripeConfig) *Client {
	return &Client{
		apiKey:        cfg.APIKey,
		webhookSecret: cfg.WebhookSecret,
		baseURL:       strings.TrimRight(cfg.APIBaseURL, "/"),
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

func NewService(cfg config.Config, repo *store.Repository, client *Client, notifier notify.Notifier, filingsService *filings.Service, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{Config: cfg, Repo: repo, Client: client, Notify: notifier, Filings: filingsService, Logger: logger}
}
