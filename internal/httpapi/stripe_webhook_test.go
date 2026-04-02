package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/stripe"
)

func TestStripeWebhookHandlerReturnsUnauthorizedForFutureSignature(t *testing.T) {
	handler := stripeWebhookHandler(discardLogger(), stripe.NewService(config.Config{}, nil, stripe.NewClient(config.StripeConfig{
		APIKey:        "sk_test",
		WebhookSecret: "whsec_test",
		APIBaseURL:    "https://api.stripe.test",
	}), nil, nil, nil, discardLogger()))

	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", strings.NewReader(`{"id":"evt_123","type":"charge.succeeded","data":{"object":{}}}`))
	req.Header.Set("Stripe-Signature", "t=9999999999,v1=deadbeef")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}
