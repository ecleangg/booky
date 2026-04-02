package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ecleangg/booky/internal/testutil"
)

func TestNewRouterServesHealthWithJSONHeaders(t *testing.T) {
	router := NewRouter(testutil.TestConfig(), nil, nil, nil, nil, nil, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("unexpected X-Content-Type-Options header %q", rr.Header().Get("X-Content-Type-Options"))
	}
	if !strings.Contains(rr.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("unexpected content type %q", rr.Header().Get("Content-Type"))
	}

	var payload map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("unexpected payload %#v", payload)
	}
}

func TestNewRouterProtectsAdminRoute(t *testing.T) {
	router := NewRouter(testutil.TestConfig(), nil, nil, nil, nil, nil, discardLogger())

	req := httptest.NewRequest(http.MethodPost, "/admin/runs/daily-close", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestDailyCloseHandlerRejectsInvalidDate(t *testing.T) {
	handler := dailyCloseHandler(testutil.TestConfig(), discardLogger(), nil)

	req := httptest.NewRequest(http.MethodPost, "/admin/runs/daily-close?date=not-a-date", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "invalid date query param") {
		t.Fatalf("unexpected response %s", rr.Body.String())
	}
}

func TestFilingStatusHandlerRequiresKindAndPeriod(t *testing.T) {
	handler := filingStatusHandler(discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/filings", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "kind and period query params are required") {
		t.Fatalf("unexpected response %s", rr.Body.String())
	}
}

func TestStripeWebhookHandlerRejectsWrongMethod(t *testing.T) {
	handler := stripeWebhookHandler(discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/webhooks/stripe", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
	if rr.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("unexpected allow header %q", rr.Header().Get("Allow"))
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
