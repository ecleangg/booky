package testutil

import (
	"github.com/ecleangg/booky/internal/config"
	"github.com/google/uuid"
)

func TestConfig() config.Config {
	return config.Config{
		App:      config.AppConfig{Env: "test", Timezone: "Europe/Stockholm"},
		HTTP:     config.HTTPConfig{ListenAddr: "127.0.0.1:0", ReadTimeout: "1s", WriteTimeout: "1s", IdleTimeout: "1s"},
		Postgres: config.PostgresConfig{DSN: "postgresql://unused"},
		Admin:    config.AdminConfig{Enabled: true, BearerToken: "secret-token"},
		Stripe:   config.StripeConfig{APIKey: "sk_test", WebhookSecret: "whsec_test", APIBaseURL: "https://api.stripe.test"},
		Bokio:    config.BokioConfig{CompanyID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Token: "bokio-token", BaseURL: "https://api.bokio.test/v1"},
		Posting: config.PostingConfig{
			CutoffTime:            "23:59",
			SchedulerEnabled:      false,
			SchedulerPollInterval: "1m",
			AutoPostUnknownToOBS:  false,
			RoundingToleranceOre:  2,
		},
		Notifications: config.NotificationsConfig{
			Resend: config.ResendConfig{
				Enabled:       true,
				APIKey:        "resend-key",
				From:          "bookkeeping@example.com",
				To:            []string{"finance@example.com"},
				BaseURL:       "https://api.resend.test",
				SubjectPrefix: "[booky]",
			},
		},
		Filings: config.FilingsConfig{
			Enabled:       true,
			LeadTimeDays:  7,
			SendTimeLocal: "09:00",
			EmailTo:       []string{"finance@example.com"},
			OSSUnion: config.OSSUnionFilingsConfig{
				Enabled:          true,
				IdentifierNumber: "SE556000016701",
				OriginCountry:    "SE",
				ZeroSalesPolicy:  "reminder_only",
			},
			PeriodicSummary: config.PeriodicSummaryFilingsConfig{
				Enabled:            true,
				Cadence:            "monthly",
				ReportingVATNumber: "SE556000016701",
				ResponsibleName:    "eclean Finance",
				ResponsiblePhone:   "+46701234567",
				ResponsibleEmail:   "finance@eclean.gg",
			},
		},
		Accounts: config.AccountsConfig{
			StripeReceivable:        1580,
			Bank:                    1920,
			Dispute:                 1510,
			FallbackOBS:             2999,
			Rounding:                3740,
			StripeBalanceByCurrency: map[string]int{"SEK": 1980, "EUR": 1981, "USD": 1982},
			StripeFees:              config.StripeFeesConfig{Expense: 4535, InputVAT: 2645, OutputVAT: 2614},
			SalesByMarket: map[string]config.SalesMarketConfig{
				"SE":     {Revenue: 3001, OutputVAT: 2611, VATRatePercent: 25},
				"DE":     {Revenue: 3105, OutputVAT: 2614, VATRatePercent: 19},
				"EU_B2B": {Revenue: 3308},
				"EXPORT": {Revenue: 3305},
			},
			OtherCountriesDefault: &config.SalesMarketConfig{Revenue: 3100, OutputVAT: 2614, VATRatePercent: 25},
		},
	}
}
