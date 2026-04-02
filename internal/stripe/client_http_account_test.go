package stripe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ecleangg/booky/internal/config"
)

func TestClientWithAccountSetsStripeAccountHeader(t *testing.T) {
	var gotAccount string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccount = r.Header.Get("Stripe-Account")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"pi_123","amount":1000,"currency":"sek","customer":"cus_123","metadata":{}}`))
	}))
	defer server.Close()

	client := NewClient(config.StripeConfig{
		APIKey:     "sk_test",
		APIBaseURL: server.URL,
	}).WithAccount("acct_123")

	intent, _, err := client.GetPaymentIntent(context.Background(), "pi_123")
	if err != nil {
		t.Fatalf("GetPaymentIntent returned error: %v", err)
	}
	if intent.ID != "pi_123" {
		t.Fatalf("unexpected payment intent id %q", intent.ID)
	}
	if gotAccount != "acct_123" {
		t.Fatalf("expected Stripe-Account header acct_123, got %q", gotAccount)
	}
}
