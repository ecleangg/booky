package integrations

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ecleangg/booky/internal/config"
	"github.com/google/uuid"
)

type CompanySettings struct {
	Posting       config.PostingConfig       `json:"posting"`
	Accounts      config.AccountsConfig      `json:"accounts"`
	Notifications config.NotificationsConfig `json:"notifications"`
	Filings       config.FilingsConfig       `json:"filings"`
}

type RuntimeConfig struct {
	WorkspaceID      string
	PairingID        uuid.UUID
	StripeAccountID  string
	Livemode         bool
	BokioCompanyID   uuid.UUID
	BokioCompanyName string
	BokioToken       string
	Settings         CompanySettings
}

func DefaultCompanySettings(cfg config.Config) CompanySettings {
	return CompanySettings{
		Posting:       cfg.Posting,
		Accounts:      cfg.Accounts,
		Notifications: cfg.Notifications,
		Filings:       cfg.Filings,
	}
}

func ParseCompanySettings(raw json.RawMessage, defaults CompanySettings) (CompanySettings, error) {
	settings := defaults
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "" || string(raw) == "{}" {
		return settings, nil
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return CompanySettings{}, fmt.Errorf("decode company settings: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err == nil {
		if _, hasFilings := fields["filings"]; !hasFilings {
			var legacy struct {
				OSSReportingEnabled *bool `json:"oss_reporting_enabled"`
			}
			if err := json.Unmarshal(raw, &legacy); err == nil && legacy.OSSReportingEnabled != nil {
				settings.Filings.OSSUnion.Enabled = *legacy.OSSReportingEnabled
			}
		}
	}
	return settings, nil
}

func ValidateCompanySettings(settings CompanySettings) error {
	var errs []string
	errs = append(errs, config.ValidatePostingConfig(settings.Posting)...)
	errs = append(errs, config.ValidateAccountsConfig(settings.Accounts)...)
	errs = append(errs, config.ValidateNotificationsConfig(settings.Notifications)...)
	errs = append(errs, config.ValidateFilingsConfig(settings.Filings, settings.Notifications)...)
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (r RuntimeConfig) Apply(base config.Config) config.Config {
	cfg := base
	cfg.Bokio.CompanyID = r.BokioCompanyID
	cfg.Bokio.Token = r.BokioToken
	cfg.Posting = r.Settings.Posting
	cfg.Accounts = r.Settings.Accounts
	cfg.Notifications = r.Settings.Notifications
	cfg.Filings = r.Settings.Filings
	return cfg
}
