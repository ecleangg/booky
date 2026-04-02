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
	if cfg.LegacyRuntimeEnabled() {
		if cfg.Bokio.CompanyID == uuid.Nil {
			errs = append(errs, "bokio.company_id is required")
		}
		if strings.TrimSpace(cfg.Bokio.Token) == "" {
			errs = append(errs, "bokio.token is required")
		}
	} else if !cfg.WorkspacePairingsEnabled() {
		errs = append(errs, "either legacy bokio credentials or bokio.oauth configuration is required")
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
	if cfg.Auth.JWT.Enabled {
		if strings.TrimSpace(cfg.Auth.JWT.Issuer) == "" {
			errs = append(errs, "auth.jwt.issuer is required when auth.jwt.enabled is true")
		}
		if strings.TrimSpace(cfg.Auth.JWT.Audience) == "" {
			errs = append(errs, "auth.jwt.audience is required when auth.jwt.enabled is true")
		}
		if strings.TrimSpace(cfg.Auth.JWT.JWKSURL) == "" {
			errs = append(errs, "auth.jwt.jwks_url is required when auth.jwt.enabled is true")
		}
	}
	if cfg.WorkspacePairingsEnabled() {
		if !cfg.Auth.JWT.Enabled {
			errs = append(errs, "auth.jwt.enabled must be true when workspace pairings are configured")
		}
		if strings.TrimSpace(cfg.Stripe.Connect.ClientID) == "" {
			errs = append(errs, "stripe.connect.client_id is required when workspace pairings are configured")
		}
		if strings.TrimSpace(cfg.Stripe.Connect.RedirectURI) == "" {
			errs = append(errs, "stripe.connect.redirect_uri is required when workspace pairings are configured")
		}
		if strings.TrimSpace(cfg.Stripe.Connect.SuccessURL) == "" {
			errs = append(errs, "stripe.connect.success_url is required when workspace pairings are configured")
		}
		if strings.TrimSpace(cfg.Stripe.Connect.ErrorURL) == "" {
			errs = append(errs, "stripe.connect.error_url is required when workspace pairings are configured")
		}
		if strings.TrimSpace(cfg.Stripe.ConnectWebhookSecret) == "" {
			errs = append(errs, "stripe.connect_webhook_secret is required when workspace pairings are configured")
		}
		if strings.TrimSpace(cfg.Bokio.OAuth.ClientID) == "" {
			errs = append(errs, "bokio.oauth.client_id is required when workspace pairings are configured")
		}
		if strings.TrimSpace(cfg.Bokio.OAuth.ClientSecret) == "" {
			errs = append(errs, "bokio.oauth.client_secret is required when workspace pairings are configured")
		}
		if strings.TrimSpace(cfg.Bokio.OAuth.RedirectURI) == "" {
			errs = append(errs, "bokio.oauth.redirect_uri is required when workspace pairings are configured")
		}
		if strings.TrimSpace(cfg.Bokio.OAuth.SuccessURL) == "" {
			errs = append(errs, "bokio.oauth.success_url is required when workspace pairings are configured")
		}
		if strings.TrimSpace(cfg.Bokio.OAuth.ErrorURL) == "" {
			errs = append(errs, "bokio.oauth.error_url is required when workspace pairings are configured")
		}
		if strings.TrimSpace(cfg.Security.DataEncryptionKey) == "" {
			errs = append(errs, "security.data_encryption_key is required when workspace pairings are configured")
		}
	}
	if _, err := time.ParseDuration(cfg.Posting.SchedulerPollInterval); err != nil {
		errs = append(errs, fmt.Sprintf("posting.scheduler_poll_interval is invalid: %v", err))
	}

	errs = append(errs, ValidateAccountsConfig(cfg.Accounts)...)

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

func ValidateAccountsConfig(accounts AccountsConfig) []string {
	var errs []string

	requiredAccounts := map[string]int{
		"accounts.stripe_receivable":      accounts.StripeReceivable,
		"accounts.bank":                   accounts.Bank,
		"accounts.dispute":                accounts.Dispute,
		"accounts.fallback_obs":           accounts.FallbackOBS,
		"accounts.rounding":               accounts.Rounding,
		"accounts.stripe_fees.expense":    accounts.StripeFees.Expense,
		"accounts.stripe_fees.input_vat":  accounts.StripeFees.InputVAT,
		"accounts.stripe_fees.output_vat": accounts.StripeFees.OutputVAT,
	}
	for name, value := range requiredAccounts {
		if value == 0 {
			errs = append(errs, name+" is required")
		}
	}

	if len(accounts.StripeBalanceByCurrency) == 0 {
		errs = append(errs, "accounts.stripe_balance_by_currency must contain at least one currency")
	}
	for currency, account := range accounts.StripeBalanceByCurrency {
		if len(strings.TrimSpace(currency)) != 3 || strings.ToUpper(currency) != currency {
			errs = append(errs, fmt.Sprintf("accounts.stripe_balance_by_currency.%s must be uppercase ISO-4217", currency))
		}
		if account == 0 {
			errs = append(errs, fmt.Sprintf("accounts.stripe_balance_by_currency.%s is required", currency))
		}
	}

	if len(accounts.SalesByMarket) == 0 {
		errs = append(errs, "accounts.sales_by_market must contain at least one market")
	}
	for market, accountCfg := range accounts.SalesByMarket {
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
	if accounts.OtherCountriesDefault != nil {
		if accounts.OtherCountriesDefault.Revenue == 0 {
			errs = append(errs, "accounts.other_countries_default.revenue is required")
		}
		if accounts.OtherCountriesDefault.OutputVAT == 0 {
			errs = append(errs, "accounts.other_countries_default.output_vat is required")
		}
		if accounts.OtherCountriesDefault.VATRatePercent <= 0 {
			errs = append(errs, "accounts.other_countries_default.vat_rate_percent must be greater than 0")
		}
	}

	return errs
}

func (c Config) SnapshotJSON() (json.RawMessage, error) {
	snapshot := c
	snapshot.Admin.BearerToken = redactSecret(snapshot.Admin.BearerToken)
	snapshot.Stripe.APIKey = redactSecret(snapshot.Stripe.APIKey)
	snapshot.Stripe.WebhookSecret = redactSecret(snapshot.Stripe.WebhookSecret)
	snapshot.Stripe.ConnectWebhookSecret = redactSecret(snapshot.Stripe.ConnectWebhookSecret)
	snapshot.Bokio.Token = redactSecret(snapshot.Bokio.Token)
	snapshot.Bokio.OAuth.ClientSecret = redactSecret(snapshot.Bokio.OAuth.ClientSecret)
	snapshot.Security.DataEncryptionKey = redactSecret(snapshot.Security.DataEncryptionKey)
	snapshot.Notifications.Resend.APIKey = redactSecret(snapshot.Notifications.Resend.APIKey)

	b, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func redactSecret(value string) string {
	if strings.TrimSpace(value) == "" {
		return value
	}
	return "***"
}
