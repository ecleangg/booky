package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type Config struct {
	App           AppConfig           `yaml:"app" json:"app"`
	HTTP          HTTPConfig          `yaml:"http" json:"http"`
	Admin         AdminConfig         `yaml:"admin" json:"admin"`
	Auth          AuthConfig          `yaml:"auth" json:"auth"`
	Security      SecurityConfig      `yaml:"security" json:"security"`
	Postgres      PostgresConfig      `yaml:"postgres" json:"postgres"`
	Stripe        StripeConfig        `yaml:"stripe" json:"stripe"`
	Bokio         BokioConfig         `yaml:"bokio" json:"bokio"`
	Posting       PostingConfig       `yaml:"posting" json:"posting"`
	Accounts      AccountsConfig      `yaml:"accounts" json:"accounts"`
	Notifications NotificationsConfig `yaml:"notifications" json:"notifications"`
	Filings       FilingsConfig       `yaml:"filings" json:"filings"`
}

type AppConfig struct {
	Env      string `yaml:"env" json:"env"`
	Timezone string `yaml:"timezone" json:"timezone"`
}

type HTTPConfig struct {
	ListenAddr   string `yaml:"listen_addr" json:"listen_addr"`
	ReadTimeout  string `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout  string `yaml:"idle_timeout" json:"idle_timeout"`
}

type AdminConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	BearerToken string `yaml:"bearer_token" json:"bearer_token"`
}

type AuthConfig struct {
	JWT JWTConfig `yaml:"jwt" json:"jwt"`
}

type JWTConfig struct {
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	Issuer         string `yaml:"issuer" json:"issuer"`
	Audience       string `yaml:"audience" json:"audience"`
	JWKSURL        string `yaml:"jwks_url" json:"jwks_url"`
	SubjectClaim   string `yaml:"subject_claim" json:"subject_claim"`
	WorkspaceClaim string `yaml:"workspace_claim" json:"workspace_claim"`
}

type SecurityConfig struct {
	DataEncryptionKey string `yaml:"data_encryption_key" json:"data_encryption_key"`
}

type PostgresConfig struct {
	DSN string `yaml:"dsn" json:"dsn"`
}

type StripeConfig struct {
	APIKey               string              `yaml:"api_key" json:"api_key"`
	WebhookSecret        string              `yaml:"webhook_secret" json:"webhook_secret"`
	ConnectWebhookSecret string              `yaml:"connect_webhook_secret" json:"connect_webhook_secret"`
	AccountID            string              `yaml:"account_id" json:"account_id"`
	APIBaseURL           string              `yaml:"api_base_url" json:"api_base_url"`
	Connect              StripeConnectConfig `yaml:"connect" json:"connect"`
}

type StripeConnectConfig struct {
	ClientID       string `yaml:"client_id" json:"client_id"`
	RedirectURI    string `yaml:"redirect_uri" json:"redirect_uri"`
	AuthorizeURL   string `yaml:"authorize_url" json:"authorize_url"`
	TokenURL       string `yaml:"token_url" json:"token_url"`
	DeauthorizeURL string `yaml:"deauthorize_url" json:"deauthorize_url"`
	SuccessURL     string `yaml:"success_url" json:"success_url"`
	ErrorURL       string `yaml:"error_url" json:"error_url"`
	Scope          string `yaml:"scope" json:"scope"`
}

type BokioConfig struct {
	CompanyID uuid.UUID        `yaml:"company_id" json:"company_id"`
	Token     string           `yaml:"token" json:"token"`
	BaseURL   string           `yaml:"base_url" json:"base_url"`
	OAuth     BokioOAuthConfig `yaml:"oauth" json:"oauth"`
}

type BokioOAuthConfig struct {
	ClientID       string `yaml:"client_id" json:"client_id"`
	ClientSecret   string `yaml:"client_secret" json:"client_secret"`
	RedirectURI    string `yaml:"redirect_uri" json:"redirect_uri"`
	AuthorizeURL   string `yaml:"authorize_url" json:"authorize_url"`
	TokenURL       string `yaml:"token_url" json:"token_url"`
	GeneralBaseURL string `yaml:"general_base_url" json:"general_base_url"`
	SuccessURL     string `yaml:"success_url" json:"success_url"`
	ErrorURL       string `yaml:"error_url" json:"error_url"`
	Scope          string `yaml:"scope" json:"scope"`
}

type PostingConfig struct {
	CutoffTime            string `yaml:"cutoff_time" json:"cutoff_time"`
	SchedulerEnabled      bool   `yaml:"scheduler_enabled" json:"scheduler_enabled"`
	SchedulerPollInterval string `yaml:"scheduler_poll_interval" json:"scheduler_poll_interval"`
	AutoPostUnknownToOBS  bool   `yaml:"auto_post_unknown_to_obs" json:"auto_post_unknown_to_obs"`
	RoundingToleranceOre  int64  `yaml:"rounding_tolerance_ore" json:"rounding_tolerance_ore"`
}

type AccountsConfig struct {
	StripeReceivable        int                          `yaml:"stripe_receivable" json:"stripe_receivable"`
	Bank                    int                          `yaml:"bank" json:"bank"`
	Dispute                 int                          `yaml:"dispute" json:"dispute"`
	FallbackOBS             int                          `yaml:"fallback_obs" json:"fallback_obs"`
	Rounding                int                          `yaml:"rounding" json:"rounding"`
	StripeBalanceByCurrency map[string]int               `yaml:"stripe_balance_by_currency" json:"stripe_balance_by_currency"`
	StripeFees              StripeFeesConfig             `yaml:"stripe_fees" json:"stripe_fees"`
	SalesByMarket           map[string]SalesMarketConfig `yaml:"sales_by_market" json:"sales_by_market"`
	OtherCountriesDefault   *SalesMarketConfig           `yaml:"other_countries_default" json:"other_countries_default"`
}

type StripeFeesConfig struct {
	Expense   int `yaml:"expense" json:"expense"`
	InputVAT  int `yaml:"input_vat" json:"input_vat"`
	OutputVAT int `yaml:"output_vat" json:"output_vat"`
}

type SalesMarketConfig struct {
	Revenue        int     `yaml:"revenue" json:"revenue"`
	OutputVAT      int     `yaml:"output_vat" json:"output_vat"`
	VATRatePercent float64 `yaml:"vat_rate_percent" json:"vat_rate_percent"`
}

type NotificationsConfig struct {
	Resend ResendConfig `yaml:"resend" json:"resend"`
}

type ResendConfig struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	APIKey        string   `yaml:"api_key" json:"api_key"`
	From          string   `yaml:"from" json:"from"`
	To            []string `yaml:"to" json:"to"`
	BaseURL       string   `yaml:"base_url" json:"base_url"`
	SubjectPrefix string   `yaml:"subject_prefix" json:"subject_prefix"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	setDefaults(&cfg)
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.App.Env == "" {
		cfg.App.Env = "dev"
	}
	if cfg.App.Timezone == "" {
		cfg.App.Timezone = "Europe/Stockholm"
	}
	if cfg.HTTP.ListenAddr == "" {
		cfg.HTTP.ListenAddr = ":8080"
	}
	if cfg.HTTP.ReadTimeout == "" {
		cfg.HTTP.ReadTimeout = "15s"
	}
	if cfg.HTTP.WriteTimeout == "" {
		cfg.HTTP.WriteTimeout = "30s"
	}
	if cfg.HTTP.IdleTimeout == "" {
		cfg.HTTP.IdleTimeout = "60s"
	}
	if cfg.Stripe.APIBaseURL == "" {
		cfg.Stripe.APIBaseURL = "https://api.stripe.com"
	}
	if cfg.Stripe.Connect.AuthorizeURL == "" {
		cfg.Stripe.Connect.AuthorizeURL = "https://connect.stripe.com/oauth/authorize"
	}
	if cfg.Stripe.Connect.TokenURL == "" {
		cfg.Stripe.Connect.TokenURL = "https://connect.stripe.com/oauth/token"
	}
	if cfg.Stripe.Connect.DeauthorizeURL == "" {
		cfg.Stripe.Connect.DeauthorizeURL = "https://connect.stripe.com/oauth/deauthorize"
	}
	if cfg.Stripe.Connect.Scope == "" {
		cfg.Stripe.Connect.Scope = "read_write"
	}
	if cfg.Bokio.BaseURL == "" {
		cfg.Bokio.BaseURL = "https://api.bokio.se/v1"
	}
	if cfg.Bokio.OAuth.GeneralBaseURL == "" {
		cfg.Bokio.OAuth.GeneralBaseURL = "https://api.bokio.se/v1"
	}
	if cfg.Bokio.OAuth.AuthorizeURL == "" {
		cfg.Bokio.OAuth.AuthorizeURL = "https://api.bokio.se/v1/authorize"
	}
	if cfg.Bokio.OAuth.TokenURL == "" {
		cfg.Bokio.OAuth.TokenURL = "https://api.bokio.se/v1/token"
	}
	if cfg.Bokio.OAuth.Scope == "" {
		cfg.Bokio.OAuth.Scope = strings.Join([]string{
			"company-information:read",
			"chart-of-accounts:read",
			"fiscal-years:read",
			"journal-entries:read",
			"journal-entries:write",
			"uploads:read",
			"uploads:write",
		}, " ")
	}
	if cfg.Auth.JWT.SubjectClaim == "" {
		cfg.Auth.JWT.SubjectClaim = "sub"
	}
	if cfg.Auth.JWT.WorkspaceClaim == "" {
		cfg.Auth.JWT.WorkspaceClaim = "workspace_id"
	}
	if cfg.Posting.CutoffTime == "" {
		cfg.Posting.CutoffTime = "23:59"
	}
	if cfg.Posting.SchedulerPollInterval == "" {
		cfg.Posting.SchedulerPollInterval = "5m"
	}
	if cfg.Posting.RoundingToleranceOre == 0 {
		cfg.Posting.RoundingToleranceOre = 2
	}
	if cfg.Accounts.StripeBalanceByCurrency == nil {
		cfg.Accounts.StripeBalanceByCurrency = map[string]int{}
	}
	if cfg.Accounts.SalesByMarket == nil {
		cfg.Accounts.SalesByMarket = map[string]SalesMarketConfig{}
	}
	if cfg.Notifications.Resend.BaseURL == "" {
		cfg.Notifications.Resend.BaseURL = "https://api.resend.com"
	}
	if cfg.Notifications.Resend.SubjectPrefix == "" {
		cfg.Notifications.Resend.SubjectPrefix = "[booky]"
	}
	if cfg.Filings.LeadTimeDays == 0 && !cfg.Filings.leadTimeDaysSet {
		cfg.Filings.LeadTimeDays = 7
	}
	if cfg.Filings.SendTimeLocal == "" {
		cfg.Filings.SendTimeLocal = "09:00"
	}
	if len(cfg.Filings.EmailTo) == 0 && len(cfg.Notifications.Resend.To) > 0 {
		cfg.Filings.EmailTo = append([]string(nil), cfg.Notifications.Resend.To...)
	}
	if cfg.Filings.OSSUnion.OriginCountry == "" {
		cfg.Filings.OSSUnion.OriginCountry = "SE"
	}
	if cfg.Filings.OSSUnion.ZeroSalesPolicy == "" {
		cfg.Filings.OSSUnion.ZeroSalesPolicy = "reminder_only"
	}
	if cfg.Filings.PeriodicSummary.Cadence == "" {
		cfg.Filings.PeriodicSummary.Cadence = "monthly"
	}
}

func (c Config) LegacyRuntimeEnabled() bool {
	return c.Bokio.CompanyID != uuid.Nil && strings.TrimSpace(c.Bokio.Token) != ""
}

func (c Config) WorkspacePairingsEnabled() bool {
	return c.Auth.JWT.Enabled ||
		strings.TrimSpace(c.Stripe.Connect.ClientID) != "" ||
		strings.TrimSpace(c.Bokio.OAuth.ClientID) != ""
}

func (c Config) Location() (*time.Location, error) {
	return time.LoadLocation(c.App.Timezone)
}

func (c Config) ReadTimeout() time.Duration {
	return mustDuration(c.HTTP.ReadTimeout)
}

func (c Config) WriteTimeout() time.Duration {
	return mustDuration(c.HTTP.WriteTimeout)
}

func (c Config) IdleTimeout() time.Duration {
	return mustDuration(c.HTTP.IdleTimeout)
}

func (c Config) SchedulerInterval() time.Duration {
	return mustDuration(c.Posting.SchedulerPollInterval)
}

func (c Config) CutoffHourMinute() (int, int, error) {
	parts := strings.Split(c.Posting.CutoffTime, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid cutoff_time %q", c.Posting.CutoffTime)
	}

	hour, err := time.Parse("15:04", c.Posting.CutoffTime)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid cutoff_time %q: %w", c.Posting.CutoffTime, err)
	}

	return hour.Hour(), hour.Minute(), nil
}

func (c Config) FilingsSendHourMinute() (int, int, error) {
	parts := strings.Split(c.Filings.SendTimeLocal, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid filings.send_time_local %q", c.Filings.SendTimeLocal)
	}

	tm, err := time.Parse("15:04", c.Filings.SendTimeLocal)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid filings.send_time_local %q: %w", c.Filings.SendTimeLocal, err)
	}

	return tm.Hour(), tm.Minute(), nil
}

func mustDuration(value string) time.Duration {
	d, err := time.ParseDuration(value)
	if err != nil {
		panic(err)
	}
	return d
}
