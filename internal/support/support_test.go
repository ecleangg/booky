package support

import (
	"testing"
	"time"

	"github.com/ecleangg/booky/internal/config"
)

func TestMetadataHelpers(t *testing.T) {
	merged := MergeStringMaps(
		map[string]string{"sale_category": "services", "customer_vat_id": ""},
		map[string]string{"customer_vat_id": "DE123456789", "flag": "yes"},
	)

	if got := MapString(merged, "missing", "customer_vat_id"); got != "DE123456789" {
		t.Fatalf("unexpected mapped string %q", got)
	}
	if !MapTruthy(merged, "flag") {
		t.Fatal("expected truthy flag")
	}
	if ParseBool("not-bool") {
		t.Fatal("expected ParseBool to reject invalid values")
	}
	if !TruthyString("Y") {
		t.Fatal("expected TruthyString to handle Y")
	}
}

func TestGeographyHelpers(t *testing.T) {
	if got := NormalizeCountry(" se "); got != "SE" {
		t.Fatalf("unexpected normalized country %q", got)
	}
	if got := CountryPrefix("de123456789"); got != "DE" {
		t.Fatalf("unexpected country prefix %q", got)
	}
	if !IsGoodsCategory("physical_goods") {
		t.Fatal("expected physical_goods to be a goods category")
	}
	if !IsEUCountry("de") {
		t.Fatal("expected DE to be an EU country")
	}
	if IsEUCountry("US") {
		t.Fatal("expected US to be outside EU helper list")
	}
}

func TestNumericAndTimeHelpers(t *testing.T) {
	if got := AbsInt64(-12); got != 12 {
		t.Fatalf("unexpected abs value %d", got)
	}
	if got := MaxInt64(10, 20); got != 20 {
		t.Fatalf("unexpected max value %d", got)
	}

	location := LocationOrUTC(config.Config{App: config.AppConfig{Timezone: "Europe/Stockholm"}})
	if location == time.UTC {
		t.Fatal("expected configured location")
	}

	fallback := LocationOrUTC(config.Config{App: config.AppConfig{Timezone: "Bad/Timezone"}})
	if fallback != time.UTC {
		t.Fatalf("expected UTC fallback, got %v", fallback)
	}
}
