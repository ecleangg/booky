package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTaxCaseHandlerRejectsInvalidID(t *testing.T) {
	handler := taxCaseHandler(discardLogger(), nil)
	req := httptest.NewRequest(http.MethodGet, "/admin/tax/cases/not-a-uuid", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for missing service, got %d", rr.Code)
	}
}

func TestTaxCaseRebuildHandlerRequiresID(t *testing.T) {
	handler := taxCaseRebuildHandler(discardLogger(), nil)
	req := httptest.NewRequest(http.MethodPost, "/admin/tax/cases/rebuild", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for missing service, got %d", rr.Code)
	}
}
