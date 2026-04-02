package stripe

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/store"
	"github.com/ecleangg/booky/internal/testutil"
	"github.com/google/uuid"
)

func TestHandleWebhookStoresAndDeduplicatesUnknownEvent(t *testing.T) {
	repo, _ := testutil.NewTestRepository(t)
	cfg := testutil.TestConfig()
	cfg.Stripe.WebhookSecret = "whsec_test"

	service := NewService(cfg, repo, NewClient(cfg.Stripe), nil, nil, nil, nil, stripeTestLogger())
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

func TestRebindFactTaxCaseIDs(t *testing.T) {
	transientID := uuid.New()
	persistedID := uuid.New()
	otherID := uuid.New()

	facts := []domain.AccountingFact{
		{TaxCaseID: &transientID},
		{TaxCaseID: &otherID},
		{},
	}
	taxCases := []domain.TaxCase{
		{ID: transientID, RootObjectType: "charge", RootObjectID: "ch_123"},
	}
	persistedCaseIDs := map[string]uuid.UUID{
		"charge:ch_123": persistedID,
	}

	rebindFactTaxCaseIDs(facts, taxCases, persistedCaseIDs)

	if facts[0].TaxCaseID == nil || *facts[0].TaxCaseID != persistedID {
		t.Fatalf("expected first fact to be rebound to %s, got %#v", persistedID, facts[0].TaxCaseID)
	}
	if facts[1].TaxCaseID == nil || *facts[1].TaxCaseID != otherID {
		t.Fatalf("expected unrelated fact tax case id to remain %s, got %#v", otherID, facts[1].TaxCaseID)
	}
	if facts[2].TaxCaseID != nil {
		t.Fatalf("expected nil fact tax case id to remain nil, got %#v", facts[2].TaxCaseID)
	}
}

func TestBuildFromEventIgnoresInvoiceFinalized(t *testing.T) {
	service := NewService(testutil.TestConfig(), nil, nil, nil, nil, nil, nil, stripeTestLogger())

	result, err := service.buildFromEvent(context.Background(), Event{
		ID:   "evt_123",
		Type: "invoice.finalized",
	})
	if err != nil {
		t.Fatalf("buildFromEvent returned error: %v", err)
	}
	if len(result.Snapshots) != 0 || len(result.BalanceTxs) != 0 || len(result.Facts) != 0 || len(result.TaxCases) != 0 {
		t.Fatalf("expected invoice.finalized to be ignored, got %#v", result)
	}
}

func TestHandleConnectWebhookDisconnectsPairingOnDeauthorization(t *testing.T) {
	repo, _ := testutil.NewTestRepository(t)
	cfg := testutil.TestConfig()
	cfg.Stripe.ConnectWebhookSecret = "whsec_connect"

	now := time.Now().UTC()
	stripeConn := domain.StripeConnection{
		ID:              uuid.New(),
		WorkspaceID:     "ws_123",
		StripeAccountID: "acct_123",
		StripeUserID:    "acct_123",
		Livemode:        false,
		Scope:           "read_write",
		RawAccount:      []byte(`{"id":"acct_123"}`),
		Status:          domain.ConnectionStatusActive,
		ConnectedAt:     now,
		CreatedAt:       now,
	}
	bokioConn := domain.BokioConnection{
		ID:                 uuid.New(),
		WorkspaceID:        "ws_123",
		BokioConnectionID:  uuid.New(),
		BokioCompanyID:     uuid.New(),
		CompanyName:        "Acme AB",
		AccessTokenCipher:  "ciphertext",
		RefreshTokenCipher: "refresh",
		TokenExpiresAt:     now.Add(1 * time.Hour),
		Scope:              "company-information:read",
		Settings:           []byte(`{}`),
		SettingsVersion:    1,
		Status:             domain.ConnectionStatusActive,
		ConnectedAt:        now,
		CreatedAt:          now,
	}
	pairing := domain.WorkspacePairing{
		ID:                 uuid.New(),
		WorkspaceID:        "ws_123",
		StripeConnectionID: stripeConn.ID,
		BokioConnectionID:  bokioConn.ID,
		Status:             domain.PairingStatusActive,
		CreatedAt:          now,
	}

	if err := repo.InTx(context.Background(), func(q *store.Queries) error {
		if err := q.SaveStripeConnection(context.Background(), stripeConn); err != nil {
			return err
		}
		if err := q.SaveBokioConnection(context.Background(), bokioConn); err != nil {
			return err
		}
		return q.SaveWorkspacePairing(context.Background(), pairing)
	}); err != nil {
		t.Fatalf("seed integration records: %v", err)
	}

	service := NewService(cfg, repo, NewClient(cfg.Stripe), nil, nil, nil, nil, stripeTestLogger())
	payload := []byte(`{"id":"evt_deauth","type":"account.application.deauthorized","created":1775068800,"livemode":false,"account":"acct_123","data":{"object":{"id":"acct_123"}}}`)
	signature := signedHeader(payload, cfg.Stripe.ConnectWebhookSecret, time.Now().Unix())

	if err := service.HandleConnectWebhook(context.Background(), payload, signature); err != nil {
		t.Fatalf("HandleConnectWebhook returned error: %v", err)
	}

	updatedStripeConn, err := repo.Queries().GetStripeConnectionByID(context.Background(), stripeConn.ID)
	if err != nil {
		t.Fatalf("GetStripeConnectionByID returned error: %v", err)
	}
	if updatedStripeConn.Status != domain.ConnectionStatusDisconnected {
		t.Fatalf("expected stripe connection to be disconnected, got %q", updatedStripeConn.Status)
	}
	if updatedStripeConn.DisconnectedAt == nil {
		t.Fatal("expected stripe connection to have disconnected_at set")
	}

	updatedPairing, err := repo.Queries().GetWorkspacePairingRecord(context.Background(), pairing.ID)
	if err != nil {
		t.Fatalf("GetWorkspacePairingRecord returned error: %v", err)
	}
	if updatedPairing.Pairing.Status != domain.PairingStatusDisconnected {
		t.Fatalf("expected pairing to be disconnected, got %q", updatedPairing.Pairing.Status)
	}
	if updatedPairing.Pairing.DisconnectedAt == nil {
		t.Fatal("expected pairing to have disconnected_at set")
	}

	event, err := repo.Queries().GetWebhookEvent(context.Background(), "evt_deauth")
	if err != nil {
		t.Fatalf("GetWebhookEvent returned error: %v", err)
	}
	if event.ProcessedAt == nil {
		t.Fatal("expected deauthorization webhook to be marked processed")
	}
}

func stripeTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
