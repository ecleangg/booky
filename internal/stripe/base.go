package stripe

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/filings"
	"github.com/ecleangg/booky/internal/integrations"
	"github.com/ecleangg/booky/internal/notify"
	"github.com/ecleangg/booky/internal/store"
	"github.com/ecleangg/booky/internal/tax"
)

type Client struct {
	apiKey        string
	webhookSecret string
	baseURL       string
	accountID     string
	httpClient    *http.Client
}

type Service struct {
	Config  config.Config
	Repo    *store.Repository
	Client  *Client
	Tax     *tax.Service
	Notify  notify.Notifier
	Filings *filings.Service
	Tenants *integrations.Service
	Logger  *slog.Logger
}

func NewClient(cfg config.StripeConfig) *Client {
	return &Client{
		apiKey:        cfg.APIKey,
		webhookSecret: cfg.WebhookSecret,
		baseURL:       strings.TrimRight(cfg.APIBaseURL, "/"),
		accountID:     strings.TrimSpace(cfg.AccountID),
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) WithAccount(accountID string) *Client {
	if c == nil {
		return nil
	}
	copy := *c
	copy.accountID = strings.TrimSpace(accountID)
	return &copy
}

func NewService(cfg config.Config, repo *store.Repository, client *Client, taxService *tax.Service, notifier notify.Notifier, filingsService *filings.Service, tenants *integrations.Service, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{Config: cfg, Repo: repo, Client: client, Tax: taxService, Notify: notifier, Filings: filingsService, Tenants: tenants, Logger: logger}
}
