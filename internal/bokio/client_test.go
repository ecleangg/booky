package bokio

import (
	"testing"

	"github.com/ecleangg/booky/internal/config"
	"github.com/google/uuid"
)

func TestResolveURLPreservesBasePathPrefix(t *testing.T) {
	client := NewClient(config.BokioConfig{
		CompanyID: uuid.MustParse("453d7692-ff18-4d79-be6a-2519d9f6655d"),
		Token:     "test",
		BaseURL:   "https://api.bokio.se/v1",
	})

	resolved, err := client.resolveURL("/companies/453d7692-ff18-4d79-be6a-2519d9f6655d/company-information")
	if err != nil {
		t.Fatalf("resolveURL returned error: %v", err)
	}

	const want = "https://api.bokio.se/v1/companies/453d7692-ff18-4d79-be6a-2519d9f6655d/company-information"
	if resolved != want {
		t.Fatalf("resolved url mismatch\nwant: %s\ngot:  %s", want, resolved)
	}
}
