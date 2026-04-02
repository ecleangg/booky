package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestValidateAcceptsExpectedConfig(t *testing.T) {
	cfg := Config{
		App:      AppConfig{Timezone: "Europe/Stockholm"},
		HTTP:     HTTPConfig{ReadTimeout: "15s", WriteTimeout: "30s", IdleTimeout: "60s"},
		Admin:    AdminConfig{Enabled: true, BearerToken: "test-admin-token"},
		Postgres: PostgresConfig{DSN: "postgres://example"},
		Stripe:   StripeConfig{APIKey: "sk_test", WebhookSecret: "whsec_test"},
		Bokio:    BokioConfig{CompanyID: uuid.New(), Token: "bokio-token", BaseURL: "https://api.bokio.se/v1"},
		Posting:  PostingConfig{CutoffTime: "23:59", SchedulerPollInterval: "5m", RoundingToleranceOre: 2},
		Notifications: NotificationsConfig{
			Resend: ResendConfig{
				Enabled: true,
				APIKey:  "re_test",
				From:    "bookkeeping@example.com",
				To:      []string{"finance@example.com"},
				BaseURL: "https://api.resend.com",
			},
		},
		Accounts: AccountsConfig{
			StripeReceivable:        1580,
			Bank:                    1920,
			Dispute:                 1510,
			FallbackOBS:             2999,
			Rounding:                3740,
			StripeBalanceByCurrency: map[string]int{"SEK": 1980},
			StripeFees:              StripeFeesConfig{Expense: 4535, InputVAT: 2645, OutputVAT: 2614},
			SalesByMarket: map[string]SalesMarketConfig{
				"SE":     {Revenue: 3001, OutputVAT: 2611, VATRatePercent: 25},
				"EU_B2B": {Revenue: 3308},
				"EXPORT": {Revenue: 3305},
			},
		},
		Filings: FilingsConfig{
			Enabled:       true,
			LeadTimeDays:  7,
			SendTimeLocal: "09:00",
			EmailTo:       []string{"finance@example.com"},
			OSSUnion: OSSUnionFilingsConfig{
				Enabled:          true,
				IdentifierNumber: "SE556000016701",
				OriginCountry:    "SE",
				ZeroSalesPolicy:  "reminder_only",
			},
			PeriodicSummary: PeriodicSummaryFilingsConfig{
				Enabled:            true,
				Cadence:            "monthly",
				ReportingVATNumber: "SE556000016701",
				ResponsibleName:    "eclean Finance",
				ResponsiblePhone:   "+46701234567",
				ResponsibleEmail:   "finance@eclean.gg",
			},
		},
	}

	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestValidateRejectsEnabledAdminWithoutToken(t *testing.T) {
	cfg := Config{
		App:      AppConfig{Timezone: "Europe/Stockholm"},
		HTTP:     HTTPConfig{ReadTimeout: "15s", WriteTimeout: "30s", IdleTimeout: "60s"},
		Admin:    AdminConfig{Enabled: true},
		Postgres: PostgresConfig{DSN: "postgres://example"},
		Stripe:   StripeConfig{APIKey: "sk_test", WebhookSecret: "whsec_test"},
		Bokio:    BokioConfig{CompanyID: uuid.New(), Token: "bokio-token", BaseURL: "https://api.bokio.se/v1"},
		Posting:  PostingConfig{CutoffTime: "23:59", SchedulerPollInterval: "5m", RoundingToleranceOre: 2},
		Notifications: NotificationsConfig{
			Resend: ResendConfig{
				Enabled: true,
				APIKey:  "re_test",
				From:    "bookkeeping@example.com",
				To:      []string{"finance@example.com"},
				BaseURL: "https://api.resend.com",
			},
		},
		Accounts: AccountsConfig{
			StripeReceivable:        1580,
			Bank:                    1920,
			Dispute:                 1510,
			FallbackOBS:             2999,
			Rounding:                3740,
			StripeBalanceByCurrency: map[string]int{"SEK": 1980},
			StripeFees:              StripeFeesConfig{Expense: 4535, InputVAT: 2645, OutputVAT: 2614},
			SalesByMarket: map[string]SalesMarketConfig{
				"SE":     {Revenue: 3001, OutputVAT: 2611, VATRatePercent: 25},
				"EU_B2B": {Revenue: 3308},
				"EXPORT": {Revenue: 3305},
			},
		},
		Filings: FilingsConfig{
			Enabled:       true,
			LeadTimeDays:  7,
			SendTimeLocal: "09:00",
			EmailTo:       []string{"finance@example.com"},
			OSSUnion: OSSUnionFilingsConfig{
				Enabled:          true,
				IdentifierNumber: "SE556000016701",
				OriginCountry:    "SE",
				ZeroSalesPolicy:  "reminder_only",
			},
			PeriodicSummary: PeriodicSummaryFilingsConfig{
				Enabled:            true,
				Cadence:            "monthly",
				ReportingVATNumber: "SE556000016701",
				ResponsibleName:    "eclean Finance",
				ResponsiblePhone:   "+46701234567",
				ResponsibleEmail:   "finance@eclean.gg",
			},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "admin.bearer_token is required") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestLoadPreservesExplicitZeroLeadTimeDays(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "booky.yaml")
	if err := os.WriteFile(path, []byte(`
postgres:
  dsn: postgres://example
stripe:
  api_key: sk_test
  webhook_secret: whsec_test
bokio:
  company_id: 11111111-1111-1111-1111-111111111111
  token: bokio-token
accounts:
  stripe_receivable: 1580
  bank: 1920
  dispute: 1510
  fallback_obs: 2999
  rounding: 3740
  stripe_balance_by_currency:
    SEK: 1980
  stripe_fees:
    expense: 4535
    input_vat: 2645
    output_vat: 2614
  sales_by_market:
    SE:
      revenue: 3001
      output_vat: 2611
      vat_rate_percent: 25
filings:
  enabled: true
  lead_time_days: 0
  send_time_local: "09:00"
  email_to:
    - finance@example.com
  oss_union:
    enabled: true
    identifier_number: SE556000016701
    origin_country: SE
    zero_sales_policy: reminder_only
  periodic_summary:
    enabled: true
    cadence: monthly
    reporting_vat_number: SE556000016701
    responsible_name: eclean Finance
    responsible_phone: "+46701234567"
    responsible_email: finance@eclean.gg
notifications:
  resend:
    enabled: true
    api_key: re_test
    from: bookkeeping@example.com
    to:
      - finance@example.com
    base_url: https://api.resend.com
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Filings.LeadTimeDays != 0 {
		t.Fatalf("expected explicit lead_time_days=0 to be preserved, got %d", cfg.Filings.LeadTimeDays)
	}
}
