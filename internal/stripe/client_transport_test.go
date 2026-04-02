package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/testutil"
)

func TestVerifyWebhookAcceptsFixturePayload(t *testing.T) {
	payload := testutil.LoadFixture(t, "stripe", "event_charge_succeeded.json")
	client := NewClient(config.StripeConfig{
		APIKey:        "sk_test",
		WebhookSecret: "whsec_test",
		APIBaseURL:    "https://api.stripe.test",
	})

	if err := client.VerifyWebhook(payload, signedHeader(payload, "whsec_test", time.Now().Unix())); err != nil {
		t.Fatalf("VerifyWebhook returned error: %v", err)
	}
}

func TestGetInvoiceExpandsProductFieldsAndAcceptsStringProducts(t *testing.T) {
	fixture := []byte(`{
		"id":"in_123",
		"customer":"cus_123",
		"currency":"eur",
		"subtotal":11900,
		"total":11900,
		"lines":{
			"data":[
				{
					"taxes":[],
					"price":{"id":"price_123","type":"one_time","product":"prod_price"},
					"pricing":{"price_details":{"product":"prod_pricing"}}
				}
			]
		}
	}`)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if r.URL.Path != "/v1/invoices/in_123" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		query := r.URL.Query()["expand[]"]
		if !contains(query, "lines.data.taxes") {
			t.Fatalf("missing taxes expand, got %v", query)
		}
		if !contains(query, "lines.data.price.product") {
			t.Fatalf("missing price product expand, got %v", query)
		}
		if !contains(query, "lines.data.pricing.price_details.product") {
			t.Fatalf("missing pricing product expand, got %v", query)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := NewClient(config.StripeConfig{
		APIKey:        "sk_test",
		WebhookSecret: "whsec_test",
		APIBaseURL:    server.URL + "/v1",
	})

	invoice, raw, err := client.GetInvoice(context.Background(), "in_123")
	if err != nil {
		t.Fatalf("GetInvoice returned error: %v", err)
	}
	if invoice.ID != "in_123" {
		t.Fatalf("unexpected invoice id %q", invoice.ID)
	}
	if got := invoice.Lines.Data[0].Price.Product.ID; got != "prod_price" {
		t.Fatalf("unexpected price product id %q", got)
	}
	if got := invoice.Lines.Data[0].Pricing.PriceDetails.Product.ID; got != "prod_pricing" {
		t.Fatalf("unexpected pricing product id %q", got)
	}
	if !strings.Contains(string(raw), "\"in_123\"") {
		t.Fatalf("raw response missing invoice id: %s", string(raw))
	}
}

func TestVerifyWebhookRejectsFutureTimestamp(t *testing.T) {
	payload := testutil.LoadFixture(t, "stripe", "event_charge_succeeded.json")
	client := NewClient(config.StripeConfig{
		APIKey:        "sk_test",
		WebhookSecret: "whsec_test",
		APIBaseURL:    "https://api.stripe.test",
	})

	err := client.VerifyWebhook(payload, signedHeader(payload, "whsec_test", time.Now().Add(6*time.Minute).Unix()))
	if !errors.Is(err, ErrWebhookSignatureInFuture) {
		t.Fatalf("expected ErrWebhookSignatureInFuture, got %v", err)
	}
}

func TestParseEventReadsFixturePayload(t *testing.T) {
	payload := testutil.LoadFixture(t, "stripe", "event_charge_succeeded.json")
	client := NewClient(config.StripeConfig{
		APIKey:        "sk_test",
		WebhookSecret: "whsec_test",
		APIBaseURL:    "https://api.stripe.test",
	})

	event, err := client.ParseEvent(payload)
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	if event.ID != "evt_123" {
		t.Fatalf("unexpected event id %q", event.ID)
	}
	if event.Type != "charge.succeeded" {
		t.Fatalf("unexpected event type %q", event.Type)
	}
	if event.Account != "acct_123" {
		t.Fatalf("unexpected account %q", event.Account)
	}
}

func TestParseEventRejectsMissingIDOrType(t *testing.T) {
	client := NewClient(config.StripeConfig{
		APIKey:        "sk_test",
		WebhookSecret: "whsec_test",
		APIBaseURL:    "https://api.stripe.test",
	})

	_, err := client.ParseEvent([]byte(`{"id":"","type":"","data":{"object":{}}}`))
	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("expected ErrInvalidEvent, got %v", err)
	}
}

func TestGetChargeSendsAuthorizationAndParsesResponse(t *testing.T) {
	fixture := testutil.LoadFixture(t, "stripe", "charge.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if r.URL.Path != "/v1/charges/ch_123" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk_test" {
			t.Fatalf("unexpected auth header %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := NewClient(config.StripeConfig{
		APIKey:        "sk_test",
		WebhookSecret: "whsec_test",
		APIBaseURL:    server.URL + "/v1",
	})

	charge, raw, err := client.GetCharge(context.Background(), "ch_123")
	if err != nil {
		t.Fatalf("GetCharge returned error: %v", err)
	}
	if charge.ID != "ch_123" {
		t.Fatalf("unexpected charge id %q", charge.ID)
	}
	if charge.Amount != 11900 {
		t.Fatalf("unexpected amount %d", charge.Amount)
	}
	if charge.Currency != "eur" {
		t.Fatalf("unexpected currency %q", charge.Currency)
	}
	if !strings.Contains(string(raw), "\"ch_123\"") {
		t.Fatalf("raw response missing charge id: %s", string(raw))
	}
}

func TestGetChargeReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(config.StripeConfig{
		APIKey:        "sk_test",
		WebhookSecret: "whsec_test",
		APIBaseURL:    server.URL + "/v1",
	})

	_, _, err := client.GetCharge(context.Background(), "ch_123")
	if err == nil {
		t.Fatal("expected GetCharge to fail")
	}
	if !strings.Contains(err.Error(), "returned 400") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestConvertBalanceTransactionUsesExchangeRate(t *testing.T) {
	rate := 1.23
	cfg := testutil.TestConfig()
	event := Event{ID: "evt_123", Account: "acct_123"}
	transaction := BalanceTransactionAPI{
		ID:                "bt_123",
		Amount:            1000,
		Fee:               100,
		Net:               900,
		Currency:          "eur",
		Type:              "charge",
		ReportingCategory: "charge",
		Status:            "available",
		ExchangeRate:      &rate,
		Created:           1775068800,
		AvailableOn:       1775155200,
		Source:            StripeRef{ID: "ch_123", Object: "charge"},
	}

	converted := convertBalanceTransaction(cfg, event, transaction, []byte(`{"id":"bt_123"}`))

	if converted.Currency != "EUR" {
		t.Fatalf("unexpected currency %q", converted.Currency)
	}
	if valueOrZero(converted.AmountSEKOre) != 1230 {
		t.Fatalf("unexpected amount SEK %d", valueOrZero(converted.AmountSEKOre))
	}
	if valueOrZero(converted.FeeSEKOre) != 123 {
		t.Fatalf("unexpected fee SEK %d", valueOrZero(converted.FeeSEKOre))
	}
	if valueOrZero(converted.NetSEKOre) != 1107 {
		t.Fatalf("unexpected net SEK %d", valueOrZero(converted.NetSEKOre))
	}
	if converted.SourceObjectID != "ch_123" {
		t.Fatalf("unexpected source object id %q", converted.SourceObjectID)
	}
}

func TestAmountToSEKOrePrefersStoredSEKValues(t *testing.T) {
	amountSEK := int64(1500)
	feeSEK := int64(100)
	bt := domainBalanceTransaction("EUR", 1200, 80, 1120, amountSEK, feeSEK)

	if got := amountToSEKOre(1200, bt); got != 1500 {
		t.Fatalf("unexpected converted amount %d", got)
	}
	if got := amountToSEKOre(80, bt); got != 100 {
		t.Fatalf("unexpected converted fee %d", got)
	}
}

func signedHeader(payload []byte, secret string, timestamp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(fmt.Sprintf("%d.%s", timestamp, payload)))
	return fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil)))
}

func domainBalanceTransaction(currency string, amountMinor, feeMinor, netMinor, amountSEK, feeSEK int64) domain.BalanceTransaction {
	return domain.BalanceTransaction{
		Currency:     currency,
		AmountMinor:  amountMinor,
		FeeMinor:     feeMinor,
		NetMinor:     netMinor,
		AmountSEKOre: &amountSEK,
		FeeSEKOre:    &feeSEK,
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
