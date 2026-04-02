package httpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ecleangg/booky/internal/filings"
	"github.com/ecleangg/booky/internal/store"
)

func filingStatusHandler(logger *slog.Logger, service *filings.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		kind, period, ok := filingQueryParams(w, r)
		if !ok {
			return
		}
		status, err := service.GetStatus(r.Context(), kind, period)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, err)
				return
			}
			logger.Error("filing status failed", "kind", kind, "period", period, "error", err)
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, status)
	}
}

func filingRunHandler(logger *slog.Logger, service *filings.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		kind, period, ok := filingQueryParams(w, r)
		if !ok {
			return
		}
		export, err := service.RunPeriod(r.Context(), kind, period)
		if err != nil {
			logger.Error("filing run failed", "kind", kind, "period", period, "error", err)
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		status, err := service.GetStatus(r.Context(), kind, period)
		if err != nil {
			logger.Error("filing status after run failed", "kind", kind, "period", period, "error", err)
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":       "completed",
			"kind":         kind,
			"period":       period,
			"export_sent":  export != nil,
			"latest_state": status,
		})
	}
}

func filingMarkSubmittedHandler(logger *slog.Logger, service *filings.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		kind, period, ok := filingQueryParams(w, r)
		if !ok {
			return
		}
		submittedAt := time.Now()
		if raw := r.URL.Query().Get("submitted_at"); raw != "" {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, fmt.Errorf("invalid submitted_at query param: %w", err))
				return
			}
			submittedAt = parsed
		}
		if err := service.MarkSubmitted(r.Context(), kind, period, submittedAt); err != nil {
			logger.Error("filing mark submitted failed", "kind", kind, "period", period, "error", err)
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":       "submitted",
			"kind":         kind,
			"period":       period,
			"submitted_at": submittedAt.UTC().Format(time.RFC3339),
		})
	}
}

func filingQueryParams(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	kind := r.URL.Query().Get("kind")
	period := r.URL.Query().Get("period")
	if kind == "" || period == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("kind and period query params are required"))
		return "", "", false
	}
	return kind, period, true
}
