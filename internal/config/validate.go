package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

func Validate(cfg Config) error {
	var errs []string

	if strings.TrimSpace(cfg.Postgres.DSN) == "" {
		errs = append(errs, "postgres.dsn is required")
	}
	if strings.TrimSpace(cfg.Stripe.APIKey) == "" {
		errs = append(errs, "stripe.api_key is required")
	}
	if strings.TrimSpace(cfg.Stripe.WebhookSecret) == "" {
		errs = append(errs, "stripe.webhook_secret is required")
	}
	if cfg.Bokio.CompanyID == uuid.Nil {
		errs = append(errs, "bokio.company_id is required")
	}
	if strings.TrimSpace(cfg.Bokio.Token) == "" {
		errs = append(errs, "bokio.token is required")
	}

	if _, err := time.LoadLocation(cfg.App.Timezone); err != nil {
		errs = append(errs, fmt.Sprintf("app.timezone is invalid: %v", err))
	}
	if _, _, err := cfg.CutoffHourMinute(); err != nil {
		errs = append(errs, err.Error())
	}
	if _, err := time.ParseDuration(cfg.HTTP.ReadTimeout); err != nil {
		errs = append(errs, fmt.Sprintf("http.read_timeout is invalid: %v", err))
	}
	if _, err := time.ParseDuration(cfg.HTTP.WriteTimeout); err != nil {
		errs = append(errs, fmt.Sprintf("http.write_timeout is invalid: %v", err))
	}
	if _, err := time.ParseDuration(cfg.HTTP.IdleTimeout); err != nil {
		errs = append(errs, fmt.Sprintf("http.idle_timeout is invalid: %v", err))
	}
	if cfg.Admin.Enabled && strings.TrimSpace(cfg.Admin.BearerToken) == "" {
		errs = append(errs, "admin.bearer_token is required when admin.enabled is true")
	}
	if _, err := time.ParseDuration(cfg.Posting.SchedulerPollInterval); err != nil {
		errs = append(errs, fmt.Sprintf("posting.scheduler_poll_interval is invalid: %v", err))
	}

	requiredAccounts := map[string]int{
		"accounts.stripe_receivable":      cfg.Accounts.StripeReceivable,
		"accounts.bank":                   cfg.Accounts.Bank,
		"accounts.dispute":                cfg.Accounts.Dispute,
		"accounts.fallback_obs":           cfg.Accounts.FallbackOBS,
		"accounts.rounding":               cfg.Accounts.Rounding,
		"accounts.stripe_fees.expense":    cfg.Accounts.StripeFees.Expense,
		"accounts.stripe_fees.input_vat":  cfg.Accounts.StripeFees.InputVAT,
		"accounts.stripe_fees.output_vat": cfg.Accounts.StripeFees.OutputVAT,
	}
	for name, value := range requiredAccounts {
		if value == 0 {
			errs = append(errs, name+" is required")
		}
	}

	if len(cfg.Accounts.StripeBalanceByCurrency) == 0 {
		errs = append(errs, "accounts.stripe_balance_by_currency must contain at least one currency")
	}
	for currency, account := range cfg.Accounts.StripeBalanceByCurrency {
		if len(strings.TrimSpace(currency)) != 3 || strings.ToUpper(currency) != currency {
			errs = append(errs, fmt.Sprintf("accounts.stripe_balance_by_currency.%s must be uppercase ISO-4217", currency))
		}
		if account == 0 {
			errs = append(errs, fmt.Sprintf("accounts.stripe_balance_by_currency.%s is required", currency))
		}
	}

	if len(cfg.Accounts.SalesByMarket) == 0 {
		errs = append(errs, "accounts.sales_by_market must contain at least one market")
	}
	for market, accountCfg := range cfg.Accounts.SalesByMarket {
		if accountCfg.Revenue == 0 {
			errs = append(errs, fmt.Sprintf("accounts.sales_by_market.%s.revenue is required", market))
		}
		switch market {
		case "EU_B2B", "EXPORT":
			if accountCfg.OutputVAT != 0 {
				errs = append(errs, fmt.Sprintf("accounts.sales_by_market.%s.output_vat must be empty", market))
			}
		default:
			if accountCfg.OutputVAT == 0 {
				errs = append(errs, fmt.Sprintf("accounts.sales_by_market.%s.output_vat is required", market))
			}
		}
	}
	if cfg.Accounts.OtherCountriesDefault != nil {
		if cfg.Accounts.OtherCountriesDefault.Revenue == 0 {
			errs = append(errs, "accounts.other_countries_default.revenue is required")
		}
		if cfg.Accounts.OtherCountriesDefault.OutputVAT == 0 {
			errs = append(errs, "accounts.other_countries_default.output_vat is required")
		}
		if cfg.Accounts.OtherCountriesDefault.VATRatePercent <= 0 {
			errs = append(errs, "accounts.other_countries_default.vat_rate_percent must be greater than 0")
		}
	}

	if cfg.Notifications.Resend.Enabled {
		if strings.TrimSpace(cfg.Notifications.Resend.APIKey) == "" {
			errs = append(errs, "notifications.resend.api_key is required when enabled")
		}
		if strings.TrimSpace(cfg.Notifications.Resend.From) == "" {
			errs = append(errs, "notifications.resend.from is required when enabled")
		}
		if len(cfg.Notifications.Resend.To) == 0 {
			errs = append(errs, "notifications.resend.to must contain at least one recipient when enabled")
		}
		if strings.TrimSpace(cfg.Notifications.Resend.BaseURL) == "" {
			errs = append(errs, "notifications.resend.base_url is required when enabled")
		}
	}
	if cfg.Filings.Enabled {
		if cfg.Filings.LeadTimeDays < 0 {
			errs = append(errs, "filings.lead_time_days must be zero or greater")
		}
		if _, _, err := cfg.FilingsSendHourMinute(); err != nil {
			errs = append(errs, err.Error())
		}
		if !cfg.Notifications.Resend.Enabled {
			errs = append(errs, "notifications.resend.enabled must be true when filings.enabled is true")
		}
		if len(cfg.Filings.EmailTo) == 0 {
			errs = append(errs, "filings.email_to must contain at least one recipient when filings.enabled is true")
		}
		switch cfg.Filings.FX.OSSProvider {
		case "ecb_period_end":
		default:
			errs = append(errs, fmt.Sprintf("filings.fx.oss_provider %q is unsupported", cfg.Filings.FX.OSSProvider))
		}
		switch cfg.Filings.FX.PSProvider {
		case "riksbank_monthly_average":
		default:
			errs = append(errs, fmt.Sprintf("filings.fx.ps_provider %q is unsupported", cfg.Filings.FX.PSProvider))
		}
		if cfg.Filings.OSSUnion.Enabled {
			if strings.TrimSpace(cfg.Filings.OSSUnion.IdentifierNumber) == "" {
				errs = append(errs, "filings.oss_union.identifier_number is required when enabled")
			}
			if strings.ToUpper(strings.TrimSpace(cfg.Filings.OSSUnion.OriginCountry)) != "SE" {
				errs = append(errs, "filings.oss_union.origin_country must be SE in v1")
			}
			if cfg.Filings.OSSUnion.ZeroSalesPolicy != "reminder_only" {
				errs = append(errs, "filings.oss_union.zero_sales_policy must be reminder_only in v1")
			}
		}
		if cfg.Filings.PeriodicSummary.Enabled {
			if strings.TrimSpace(cfg.Filings.PeriodicSummary.ReportingVATNumber) == "" {
				errs = append(errs, "filings.periodic_summary.reporting_vat_number is required when enabled")
			}
			if strings.TrimSpace(cfg.Filings.PeriodicSummary.ResponsibleName) == "" {
				errs = append(errs, "filings.periodic_summary.responsible_name is required when enabled")
			}
			if strings.TrimSpace(cfg.Filings.PeriodicSummary.ResponsiblePhone) == "" {
				errs = append(errs, "filings.periodic_summary.responsible_phone is required when enabled")
			}
			if strings.TrimSpace(cfg.Filings.PeriodicSummary.ResponsibleEmail) == "" {
				errs = append(errs, "filings.periodic_summary.responsible_email is required when enabled")
			}
			if cfg.Filings.PeriodicSummary.Cadence != "monthly" {
				errs = append(errs, "filings.periodic_summary.cadence must be monthly in v1")
			}
		}
	}

	if len(errs) > 0 {
		sort.Strings(errs)
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

func (c Config) SnapshotJSON() (json.RawMessage, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	return b, nil
}
