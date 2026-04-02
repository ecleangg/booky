package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/integrations"
	"github.com/ecleangg/booky/internal/store"
	"github.com/google/uuid"
)

func stripeOAuthStartHandler(service *integrations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing principal"))
			return
		}
		authorizeURL, expiresAt, err := service.StartStripeOAuth(r.Context(), principal.WorkspaceID, principal.Subject)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"authorize_url": authorizeURL,
			"expires_at":    expiresAt.Format(time.RFC3339),
		})
	}
}

func stripeOAuthCallbackHandler(service *integrations.Service, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		redirectURL, err := service.CompleteStripeOAuth(
			r.Context(),
			r.URL.Query().Get("state"),
			r.URL.Query().Get("code"),
			r.URL.Query().Get("error"),
			r.URL.Query().Get("error_description"),
		)
		if err != nil {
			logger.Error("stripe oauth callback failed", "error", err)
			writeError(w, http.StatusBadRequest, err)
			return
		}
		http.Redirect(w, r, redirectURL, http.StatusFound)
	}
}

func stripeAccountsHandler(service *integrations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing principal"))
			return
		}
		records, err := service.ListStripeConnections(r.Context(), principal.WorkspaceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		items := make([]map[string]any, 0, len(records))
		for _, record := range records {
			items = append(items, map[string]any{
				"id":                record.ID,
				"stripe_account_id": record.StripeAccountID,
				"stripe_user_id":    record.StripeUserID,
				"livemode":          record.Livemode,
				"scope":             record.Scope,
				"email":             record.AccountEmail,
				"business_name":     record.BusinessName,
				"country":           record.Country,
				"status":            record.Status,
				"connected_at":      record.ConnectedAt,
				"disconnected_at":   record.DisconnectedAt,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func stripeAccountDeleteHandler(service *integrations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing principal"))
			return
		}
		if err := service.DisconnectStripeConnection(r.Context(), principal.WorkspaceID, r.PathValue("accountId")); err != nil {
			status := http.StatusInternalServerError
			if err == store.ErrNotFound {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "disconnected"})
	}
}

func bokioOAuthStartHandler(service *integrations.Service) http.HandlerFunc {
	type request struct {
		BokioTenantID *uuid.UUID `json:"bokio_tenant_id"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing principal"))
			return
		}
		var body request
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		authorizeURL, expiresAt, err := service.StartBokioOAuth(r.Context(), principal.WorkspaceID, principal.Subject, body.BokioTenantID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"authorize_url": authorizeURL,
			"expires_at":    expiresAt.Format(time.RFC3339),
		})
	}
}

func bokioOAuthCallbackHandler(service *integrations.Service, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		redirectURL, err := service.CompleteBokioOAuth(
			r.Context(),
			r.URL.Query().Get("state"),
			r.URL.Query().Get("code"),
			r.URL.Query().Get("error"),
			r.URL.Query().Get("error_description"),
		)
		if err != nil {
			logger.Error("bokio oauth callback failed", "error", err)
			writeError(w, http.StatusBadRequest, err)
			return
		}
		http.Redirect(w, r, redirectURL, http.StatusFound)
	}
}

func bokioCompaniesHandler(service *integrations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing principal"))
			return
		}
		records, err := service.ListBokioConnections(r.Context(), principal.WorkspaceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		items := make([]map[string]any, 0, len(records))
		for _, record := range records {
			settings, _ := integrations.ParseCompanySettings(record.Settings, integrations.DefaultCompanySettings(service.Config))
			items = append(items, map[string]any{
				"id":                   record.ID,
				"bokio_connection_id":  record.BokioConnectionID,
				"bokio_company_id":     record.BokioCompanyID,
				"company_name":         record.CompanyName,
				"status":               record.Status,
				"scope":                record.Scope,
				"settings_version":     record.SettingsVersion,
				"oss_reporting_enabled": settings.OSSReportingEnabled,
				"connected_at":         record.ConnectedAt,
				"disconnected_at":      record.DisconnectedAt,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func bokioCompanyHandler(service *integrations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing principal"))
			return
		}
		companyID, err := uuid.Parse(r.PathValue("companyId"))
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid company id: %w", err))
			return
		}
		record, settings, err := service.GetBokioConnection(r.Context(), principal.WorkspaceID, companyID)
		if err != nil {
			status := http.StatusInternalServerError
			if err == store.ErrNotFound {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":                   record.ID,
			"bokio_connection_id":  record.BokioConnectionID,
			"bokio_company_id":     record.BokioCompanyID,
			"company_name":         record.CompanyName,
			"status":               record.Status,
			"scope":                record.Scope,
			"settings":             settings,
			"settings_version":     record.SettingsVersion,
			"connected_at":         record.ConnectedAt,
			"disconnected_at":      record.DisconnectedAt,
		})
	}
}

func bokioCompanyChartHandler(service *integrations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing principal"))
			return
		}
		companyID, err := uuid.Parse(r.PathValue("companyId"))
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid company id: %w", err))
			return
		}
		accounts, err := service.GetBokioChartOfAccounts(r.Context(), principal.WorkspaceID, companyID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": accounts})
	}
}

func bokioCompanySettingsHandler(service *integrations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing principal"))
			return
		}
		companyID, err := uuid.Parse(r.PathValue("companyId"))
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid company id: %w", err))
			return
		}
		var settings integrations.CompanySettings
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		record, err := service.UpdateBokioCompanySettings(r.Context(), principal.WorkspaceID, companyID, settings)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":           "updated",
			"bokio_company_id": record.BokioCompanyID,
			"settings_version": record.SettingsVersion,
		})
	}
}

func bokioCompanyValidateHandler(service *integrations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing principal"))
			return
		}
		companyID, err := uuid.Parse(r.PathValue("companyId"))
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid company id: %w", err))
			return
		}
		payload, err := service.ValidateBokioConnection(r.Context(), principal.WorkspaceID, companyID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, payload)
	}
}

func bokioCompanyDeleteHandler(service *integrations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing principal"))
			return
		}
		companyID, err := uuid.Parse(r.PathValue("companyId"))
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid company id: %w", err))
			return
		}
		if err := service.DisconnectBokioConnection(r.Context(), principal.WorkspaceID, companyID); err != nil {
			status := http.StatusInternalServerError
			if err == store.ErrNotFound {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "disconnected"})
	}
}

func pairingCreateHandler(service *integrations.Service) http.HandlerFunc {
	type request struct {
		StripeAccountID string    `json:"stripe_account_id"`
		BokioCompanyID  uuid.UUID `json:"bokio_company_id"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing principal"))
			return
		}
		var body request
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		record, err := service.CreatePairing(r.Context(), principal.WorkspaceID, body.StripeAccountID, body.BokioCompanyID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, pairingPayload(record))
	}
}

func pairingsListHandler(service *integrations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing principal"))
			return
		}
		records, err := service.ListPairings(r.Context(), principal.WorkspaceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		items := make([]map[string]any, 0, len(records))
		for _, record := range records {
			items = append(items, pairingPayload(record))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func pairingGetHandler(service *integrations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing principal"))
			return
		}
		pairingID, err := uuid.Parse(r.PathValue("pairingId"))
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid pairing id: %w", err))
			return
		}
		record, err := service.GetPairing(r.Context(), principal.WorkspaceID, pairingID)
		if err != nil {
			status := http.StatusInternalServerError
			if err == store.ErrNotFound {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, pairingPayload(record))
	}
}

func pairingDeleteHandler(service *integrations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing principal"))
			return
		}
		pairingID, err := uuid.Parse(r.PathValue("pairingId"))
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid pairing id: %w", err))
			return
		}
		if err := service.DisconnectPairing(r.Context(), principal.WorkspaceID, pairingID); err != nil {
			status := http.StatusInternalServerError
			if err == store.ErrNotFound {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "disconnected"})
	}
}

func pairingValidateHandler(service *integrations.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing principal"))
			return
		}
		pairingID, err := uuid.Parse(r.PathValue("pairingId"))
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid pairing id: %w", err))
			return
		}
		payload, err := service.ValidatePairing(r.Context(), principal.WorkspaceID, pairingID)
		if err != nil {
			status := http.StatusInternalServerError
			if err == store.ErrNotFound {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, payload)
	}
}

func pairingPayload(record domain.PairingRecord) map[string]any {
	return map[string]any{
		"id":                record.Pairing.ID,
		"workspace_id":      record.Pairing.WorkspaceID,
		"status":            record.Pairing.Status,
		"created_at":        record.Pairing.CreatedAt,
		"disconnected_at":   record.Pairing.DisconnectedAt,
		"stripe_account_id": record.StripeConnection.StripeAccountID,
		"stripe_connection": map[string]any{
			"id":            record.StripeConnection.ID,
			"stripe_user_id": record.StripeConnection.StripeUserID,
			"livemode":      record.StripeConnection.Livemode,
			"scope":         record.StripeConnection.Scope,
			"email":         record.StripeConnection.AccountEmail,
			"business_name": record.StripeConnection.BusinessName,
			"country":       record.StripeConnection.Country,
			"status":        record.StripeConnection.Status,
		},
		"bokio_company_id": record.BokioConnection.BokioCompanyID,
		"bokio_connection": map[string]any{
			"id":                record.BokioConnection.ID,
			"bokio_connection_id": record.BokioConnection.BokioConnectionID,
			"company_name":      record.BokioConnection.CompanyName,
			"scope":             record.BokioConnection.Scope,
			"status":            record.BokioConnection.Status,
			"settings_version":  record.BokioConnection.SettingsVersion,
		},
	}
}
