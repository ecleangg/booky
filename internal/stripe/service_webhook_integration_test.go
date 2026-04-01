package stripe

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ecleangg/booky/internal/store"
	"github.com/ecleangg/booky/internal/testutil"
)

func TestHandleWebhookStoresAndDeduplicatesUnknownEvent(t *testing.T) {
	repo, _ := testutil.NewTestRepository(t)
	cfg := testutil.TestConfig()
	cfg.Stripe.WebhookSecret = "whsec_test"

	service := NewService(cfg, repo, NewClient(cfg.Stripe), nil, nil, stripeTestLogger())
	payload := []byte(`{"id":"evt_unknown","type":"customer.created","created":1775068800,"livemode":false,"data":{"object":{"id":"cus_123"}}}`)
	signature := signedHeader(payload, cfg.Stripe.WebhookSecret, time.Now().Unix())

	if err := service.HandleWebhook(context.Background(), payload, signature); err != nil {
		t.Fatalf("HandleWebhook returned error: %v", err)
	}

	storedEvent, err := repo.Queries().GetWebhookEvent(context.Background(), "evt_unknown")
	if err != nil {
		t.Fatalf("GetWebhookEvent returned error: %v", err)
	}
	if storedEvent.ProcessedAt == nil {
		t.Fatal("expected webhook to be marked processed")
	}

	if err := service.HandleWebhook(context.Background(), payload, signature); err != nil {
		t.Fatalf("second HandleWebhook returned error: %v", err)
	}

	storedEvent, err = repo.Queries().GetWebhookEvent(context.Background(), "evt_unknown")
	if err != nil {
		t.Fatalf("GetWebhookEvent after duplicate returned error: %v", err)
	}
	if storedEvent.ProcessingError != nil {
		t.Fatalf("expected no processing error, got %#v", storedEvent.ProcessingError)
	}

	_, err = repo.Queries().GetObjectSnapshot(context.Background(), "customer", "cus_123")
	if err == nil {
		t.Fatal("expected unknown event to skip snapshot storage")
	}
	if err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func stripeTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
