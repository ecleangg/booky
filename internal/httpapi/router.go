package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/ecleangg/booky/internal/accounting"
	"github.com/ecleangg/booky/internal/bokio"
	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/filings"
	"github.com/ecleangg/booky/internal/stripe"
)

func NewRouter(cfg config.Config, stripeService *stripe.Service, accountingService *accounting.Service, filingsService *filings.Service, bokioClient *bokio.Client, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler())
	mux.HandleFunc("/webhooks/stripe", stripeWebhookHandler(logger, stripeService))
	if cfg.Admin.Enabled {
		mux.Handle("/admin/runs/daily-close", withBearerAuth(cfg.Admin.BearerToken, dailyCloseHandler(cfg, logger, accountingService)))
		mux.Handle("/admin/runs/filing", withBearerAuth(cfg.Admin.BearerToken, filingRunHandler(logger, filingsService)))
		mux.Handle("/admin/filings", withBearerAuth(cfg.Admin.BearerToken, filingStatusHandler(logger, filingsService)))
		mux.Handle("/admin/filings/mark-submitted", withBearerAuth(cfg.Admin.BearerToken, filingMarkSubmittedHandler(logger, filingsService)))
		mux.Handle("/admin/bokio/check", withBearerAuth(cfg.Admin.BearerToken, bokioCheckHandler(logger, bokioClient)))
		mux.Handle("/admin/tax/cases/rebuild", withBearerAuth(cfg.Admin.BearerToken, taxCaseRebuildHandler(logger, stripeService)))
		mux.Handle("/admin/tax/cases/manual-evidence", withBearerAuth(cfg.Admin.BearerToken, manualTaxEvidenceHandler(logger, stripeService)))
		mux.Handle("/admin/tax/cases/", withBearerAuth(cfg.Admin.BearerToken, taxCaseHandler(logger, stripeService)))
	}

	return withJSONHeaders(withLogging(logger, mux))
}

func withLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		defer func() {
			logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started).String())
		}()
		next.ServeHTTP(w, r)
	})
}

func withJSONHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
}

func writeError(w http.ResponseWriter, status int, err error) {
	message := "request failed"
	if err != nil && status >= 400 && status < 500 {
		message = err.Error()
	} else if status >= 500 {
		message = http.StatusText(status)
	}
	writeJSON(w, status, map[string]any{"error": message})
}

func isClientError(err error) bool {
	var syntaxError *json.SyntaxError
	var typeError *json.UnmarshalTypeError
	return errors.As(err, &syntaxError) || errors.As(err, &typeError)
}
