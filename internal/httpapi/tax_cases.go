package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/store"
	"github.com/ecleangg/booky/internal/stripe"
	"github.com/google/uuid"
)

func taxCaseHandler(logger *slog.Logger, service *stripe.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, fmt.Errorf("stripe service is not configured"))
			return
		}
		id, ok := taxCaseIDFromPath(w, r)
		if !ok {
			return
		}
		payload, err := service.GetTaxCase(r.Context(), id)
		if err != nil {
			if err == store.ErrNotFound {
				writeError(w, http.StatusNotFound, err)
				return
			}
			logger.Error("get tax case failed", "tax_case_id", id, "error", err)
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, payload)
	}
}

func taxCaseRebuildHandler(logger *slog.Logger, service *stripe.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, fmt.Errorf("stripe service is not configured"))
			return
		}
		id, ok := taxCaseIDFromQuery(w, r)
		if !ok {
			return
		}
		payload, err := service.RebuildTaxCase(r.Context(), id)
		if err != nil {
			logger.Error("rebuild tax case failed", "tax_case_id", id, "error", err)
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, payload)
	}
}

func manualTaxEvidenceHandler(logger *slog.Logger, service *stripe.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, fmt.Errorf("stripe service is not configured"))
			return
		}
		var evidence domain.ManualTaxEvidence
		if err := json.NewDecoder(r.Body).Decode(&evidence); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if evidence.TaxCaseID == uuid.Nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("tax_case_id is required"))
			return
		}
		payload, err := service.RecordManualTaxEvidence(r.Context(), evidence)
		if err != nil {
			logger.Error("manual tax evidence failed", "tax_case_id", evidence.TaxCaseID, "error", err)
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, payload)
	}
}

func taxCaseIDFromPath(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := strings.TrimPrefix(r.URL.Path, "/admin/tax/cases/")
	raw = strings.TrimSpace(strings.Trim(raw, "/"))
	if raw == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("tax case id is required"))
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid tax case id: %w", err))
		return uuid.Nil, false
	}
	return id, true
}

func taxCaseIDFromQuery(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("id"))
	if raw == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("id query param is required"))
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid tax case id: %w", err))
		return uuid.Nil, false
	}
	return id, true
}
