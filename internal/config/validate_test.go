package config

import (
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
			FX: FilingFXConfig{
				OSSProvider: "ecb_period_end",
				PSProvider:  "riksbank_monthly_average",
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
			FX: FilingFXConfig{
				OSSProvider: "ecb_period_end",
				PSProvider:  "riksbank_monthly_average",
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
