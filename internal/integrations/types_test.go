package integrations

import (
	"encoding/json"
	"testing"

	"github.com/ecleangg/booky/internal/testutil"
)

func TestParseCompanySettingsSupportsLegacyOSSFlag(t *testing.T) {
	defaults := DefaultCompanySettings(testutil.TestConfig())

	settings, err := ParseCompanySettings(json.RawMessage(`{
		"accounts": {
			"bank": 1930
		},
		"oss_reporting_enabled": false
	}`), defaults)
	if err != nil {
		t.Fatalf("ParseCompanySettings returned error: %v", err)
	}

	if settings.Accounts.Bank != 1930 {
		t.Fatalf("expected bank override to be preserved, got %d", settings.Accounts.Bank)
	}
	if settings.Filings.OSSUnion.Enabled {
		t.Fatal("expected legacy oss_reporting_enabled=false to disable filings.oss_union.enabled")
	}
	if settings.Posting.CutoffTime != defaults.Posting.CutoffTime {
		t.Fatalf("expected posting defaults to be preserved, got %q", settings.Posting.CutoffTime)
	}
}

func TestParseCompanySettingsMergesNewRuntimeShape(t *testing.T) {
	defaults := DefaultCompanySettings(testutil.TestConfig())

	settings, err := ParseCompanySettings(json.RawMessage(`{
		"posting": {
			"cutoff_time": "18:30"
		},
		"notifications": {
			"resend": {
				"to": ["tenant@example.com"],
				"subject_prefix": "[tenant]"
			}
		},
		"filings": {
			"enabled": true,
			"email_to": ["tenant@example.com"],
			"oss_union": {
				"enabled": false
			}
		}
	}`), defaults)
	if err != nil {
		t.Fatalf("ParseCompanySettings returned error: %v", err)
	}

	if settings.Posting.CutoffTime != "18:30" {
		t.Fatalf("expected posting cutoff override, got %q", settings.Posting.CutoffTime)
	}
	if got := settings.Notifications.Resend.To; len(got) != 1 || got[0] != "tenant@example.com" {
		t.Fatalf("unexpected recipients %#v", got)
	}
	if settings.Notifications.Resend.APIKey != defaults.Notifications.Resend.APIKey {
		t.Fatal("expected unspecified notification fields to inherit defaults")
	}
	if settings.Filings.OSSUnion.Enabled {
		t.Fatal("expected filings override to disable OSS union")
	}
	if settings.Filings.PeriodicSummary.ReportingVATNumber != defaults.Filings.PeriodicSummary.ReportingVATNumber {
		t.Fatal("expected unspecified filings fields to inherit defaults")
	}
}

func TestRuntimeConfigApplyOverridesRuntimeSections(t *testing.T) {
	base := testutil.TestConfig()
	accounts := base.Accounts
	runtime := RuntimeConfig{
		BokioCompanyID: base.Bokio.CompanyID,
		BokioToken:     "tenant-token",
		Settings: CompanySettings{
			Posting:       base.Posting,
			Accounts:      accounts,
			Notifications: base.Notifications,
			Filings:       base.Filings,
		},
	}
	runtime.Settings.Posting.CutoffTime = "18:45"
	runtime.Settings.Accounts.Bank = 1940
	runtime.Settings.Notifications.Resend.SubjectPrefix = "[tenant]"
	runtime.Settings.Filings.EmailTo = []string{"tenant@example.com"}

	applied := runtime.Apply(base)

	if applied.Bokio.Token != "tenant-token" {
		t.Fatalf("expected Bokio token override, got %q", applied.Bokio.Token)
	}
	if applied.Posting.CutoffTime != "18:45" {
		t.Fatalf("expected posting override, got %q", applied.Posting.CutoffTime)
	}
	if applied.Accounts.Bank != 1940 {
		t.Fatalf("expected accounts override, got %d", applied.Accounts.Bank)
	}
	if applied.Notifications.Resend.SubjectPrefix != "[tenant]" {
		t.Fatalf("expected notification override, got %q", applied.Notifications.Resend.SubjectPrefix)
	}
	if got := applied.Filings.EmailTo; len(got) != 1 || got[0] != "tenant@example.com" {
		t.Fatalf("unexpected filing recipients %#v", got)
	}
}
