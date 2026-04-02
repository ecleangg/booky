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
	"github.com/ecleangg/booky/internal/integrations"
	"github.com/ecleangg/booky/internal/stripe"
)

func NewRouter(cfg config.Config, stripeService *stripe.Service, accountingService *accounting.Service, filingsService *filings.Service, bokioClient *bokio.Client, integrationsService *integrations.Service, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler())
	mux.HandleFunc("/webhooks/stripe", stripeWebhookHandler(logger, stripeService))
	mux.HandleFunc("/webhooks/stripe/connect", stripeConnectWebhookHandler(logger, stripeService))
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
	if integrationsService != nil && cfg.Auth.JWT.Enabled {
		verifier := newJWTVerifier(cfg.Auth.JWT, logger)
		protected := func(handler http.Handler) http.Handler {
			return withJWTAuth(verifier, handler)
		}
		mux.Handle("POST /v1/stripe/oauth/start", protected(stripeOAuthStartHandler(integrationsService)))
		mux.Handle("GET /v1/stripe/oauth/callback", stripeOAuthCallbackHandler(integrationsService, logger))
		mux.Handle("GET /v1/stripe/accounts", protected(stripeAccountsHandler(integrationsService)))
		mux.Handle("DELETE /v1/stripe/accounts/{accountId}", protected(stripeAccountDeleteHandler(integrationsService)))
		mux.Handle("POST /v1/bokio/oauth/start", protected(bokioOAuthStartHandler(integrationsService)))
		mux.Handle("GET /v1/bokio/oauth/callback", bokioOAuthCallbackHandler(integrationsService, logger))
		mux.Handle("GET /v1/bokio/companies", protected(bokioCompaniesHandler(integrationsService)))
		mux.Handle("GET /v1/bokio/companies/{companyId}", protected(bokioCompanyHandler(integrationsService)))
		mux.Handle("GET /v1/bokio/companies/{companyId}/chart-of-accounts", protected(bokioCompanyChartHandler(integrationsService)))
		mux.Handle("GET /v1/bokio/companies/{companyId}/settings", protected(bokioCompanySettingsGetHandler(integrationsService)))
		mux.Handle("PUT /v1/bokio/companies/{companyId}/settings", protected(bokioCompanySettingsHandler(integrationsService)))
		mux.Handle("POST /v1/bokio/companies/{companyId}/validate", protected(bokioCompanyValidateHandler(integrationsService)))
		mux.Handle("DELETE /v1/bokio/companies/{companyId}/connection", protected(bokioCompanyDeleteHandler(integrationsService)))
		mux.Handle("POST /v1/integrations/pairings", protected(pairingCreateHandler(integrationsService)))
		mux.Handle("GET /v1/integrations/pairings", protected(pairingsListHandler(integrationsService)))
		mux.Handle("GET /v1/integrations/pairings/{pairingId}", protected(pairingGetHandler(integrationsService)))
		mux.Handle("DELETE /v1/integrations/pairings/{pairingId}", protected(pairingDeleteHandler(integrationsService)))
		mux.Handle("POST /v1/integrations/pairings/{pairingId}/validate", protected(pairingValidateHandler(integrationsService)))
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
