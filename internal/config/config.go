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
	App           AppConfig           `yaml:"app"`
	HTTP          HTTPConfig          `yaml:"http"`
	Admin         AdminConfig         `yaml:"admin"`
	Postgres      PostgresConfig      `yaml:"postgres"`
	Stripe        StripeConfig        `yaml:"stripe"`
	Bokio         BokioConfig         `yaml:"bokio"`
	Posting       PostingConfig       `yaml:"posting"`
	Accounts      AccountsConfig      `yaml:"accounts"`
	Notifications NotificationsConfig `yaml:"notifications"`
	Filings       FilingsConfig       `yaml:"filings"`
}

type AppConfig struct {
	Env      string `yaml:"env"`
	Timezone string `yaml:"timezone"`
}

type HTTPConfig struct {
	ListenAddr   string `yaml:"listen_addr"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
	IdleTimeout  string `yaml:"idle_timeout"`
}

type AdminConfig struct {
	Enabled     bool   `yaml:"enabled"`
	BearerToken string `yaml:"bearer_token"`
}

type PostgresConfig struct {
	DSN string `yaml:"dsn"`
}

type StripeConfig struct {
	APIKey        string `yaml:"api_key"`
	WebhookSecret string `yaml:"webhook_secret"`
	AccountID     string `yaml:"account_id"`
	APIBaseURL    string `yaml:"api_base_url"`
}

type BokioConfig struct {
	CompanyID uuid.UUID `yaml:"company_id"`
	Token     string    `yaml:"token"`
	BaseURL   string    `yaml:"base_url"`
}

type PostingConfig struct {
	CutoffTime            string `yaml:"cutoff_time"`
	SchedulerEnabled      bool   `yaml:"scheduler_enabled"`
	SchedulerPollInterval string `yaml:"scheduler_poll_interval"`
	AutoPostUnknownToOBS  bool   `yaml:"auto_post_unknown_to_obs"`
	RoundingToleranceOre  int64  `yaml:"rounding_tolerance_ore"`
}

type AccountsConfig struct {
	StripeReceivable        int                          `yaml:"stripe_receivable"`
	Bank                    int                          `yaml:"bank"`
	Dispute                 int                          `yaml:"dispute"`
	FallbackOBS             int                          `yaml:"fallback_obs"`
	Rounding                int                          `yaml:"rounding"`
	StripeBalanceByCurrency map[string]int               `yaml:"stripe_balance_by_currency"`
	StripeFees              StripeFeesConfig             `yaml:"stripe_fees"`
	SalesByMarket           map[string]SalesMarketConfig `yaml:"sales_by_market"`
	OtherCountriesDefault   *SalesMarketConfig           `yaml:"other_countries_default"`
}

type StripeFeesConfig struct {
	Expense   int `yaml:"expense"`
	InputVAT  int `yaml:"input_vat"`
	OutputVAT int `yaml:"output_vat"`
}

type SalesMarketConfig struct {
	Revenue        int     `yaml:"revenue"`
	OutputVAT      int     `yaml:"output_vat"`
	VATRatePercent float64 `yaml:"vat_rate_percent"`
}

type NotificationsConfig struct {
	Resend ResendConfig `yaml:"resend"`
}

type ResendConfig struct {
	Enabled       bool     `yaml:"enabled"`
	APIKey        string   `yaml:"api_key"`
	From          string   `yaml:"from"`
	To            []string `yaml:"to"`
	BaseURL       string   `yaml:"base_url"`
	SubjectPrefix string   `yaml:"subject_prefix"`
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
	if cfg.Bokio.BaseURL == "" {
		cfg.Bokio.BaseURL = "https://api.bokio.se/v1"
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
	if cfg.Filings.FX.OSSProvider == "" {
		cfg.Filings.FX.OSSProvider = "ecb_period_end"
	}
	if cfg.Filings.FX.PSProvider == "" {
		cfg.Filings.FX.PSProvider = "riksbank_monthly_average"
	}
	if cfg.Filings.FX.ECBBaseURL == "" {
		cfg.Filings.FX.ECBBaseURL = "https://www.ecb.europa.eu"
	}
	if cfg.Filings.FX.RiksbankBaseURL == "" {
		cfg.Filings.FX.RiksbankBaseURL = "https://api.riksbank.se"
	}
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
