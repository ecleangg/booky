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
	Accounts           config.AccountsConfig `json:"accounts"`
	OSSReportingEnabled bool                `json:"oss_reporting_enabled"`
}

type RuntimeConfig struct {
	WorkspaceID    string
	PairingID      uuid.UUID
	StripeAccountID string
	Livemode       bool
	BokioCompanyID uuid.UUID
	BokioCompanyName string
	BokioToken     string
	Settings       CompanySettings
}

func DefaultCompanySettings(cfg config.Config) CompanySettings {
	return CompanySettings{
		Accounts:            cfg.Accounts,
		OSSReportingEnabled: cfg.Filings.OSSUnion.Enabled,
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
	return settings, nil
}

func ValidateCompanySettings(settings CompanySettings) error {
	if errs := config.ValidateAccountsConfig(settings.Accounts); len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (r RuntimeConfig) Apply(base config.Config) config.Config {
	cfg := base
	cfg.Bokio.CompanyID = r.BokioCompanyID
	cfg.Bokio.Token = r.BokioToken
	cfg.Accounts = r.Settings.Accounts
	cfg.Filings.OSSUnion.Enabled = base.Filings.OSSUnion.Enabled && r.Settings.OSSReportingEnabled
	return cfg
}
