package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/store"
	"github.com/google/uuid"
)

type stripeOAuthTokenResponse struct {
	AccessToken string `json:"access_token"`
	Livemode    bool   `json:"livemode"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
	StripeUserID string `json:"stripe_user_id"`
}

type stripeAccountPayload struct {
	ID              string `json:"id"`
	Email           string `json:"email"`
	Country         string `json:"country"`
	BusinessProfile struct {
		Name string `json:"name"`
	} `json:"business_profile"`
	Settings struct {
		Dashboard struct {
			DisplayName string `json:"display_name"`
		} `json:"dashboard"`
	} `json:"settings"`
}

func (s *Service) StartStripeOAuth(ctx context.Context, workspaceID, subject string) (string, time.Time, error) {
	if err := s.ensureEnabled(); err != nil {
		return "", time.Time{}, err
	}

	state := domain.StripeOAuthState{
		State:       uuid.NewString(),
		WorkspaceID: workspaceID,
		UserSubject: subject,
		ExpiresAt:   time.Now().UTC().Add(10 * time.Minute),
	}
	if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
		return q.CreateStripeOAuthState(ctx, state)
	}); err != nil {
		return "", time.Time{}, err
	}

	u, err := url.Parse(s.Config.Stripe.Connect.AuthorizeURL)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("parse stripe authorize url: %w", err)
	}
	query := u.Query()
	query.Set("response_type", "code")
	query.Set("client_id", s.Config.Stripe.Connect.ClientID)
	query.Set("scope", s.Config.Stripe.Connect.Scope)
	query.Set("redirect_uri", s.Config.Stripe.Connect.RedirectURI)
	query.Set("state", state.State)
	u.RawQuery = query.Encode()
	return u.String(), state.ExpiresAt, nil
}

func (s *Service) CompleteStripeOAuth(ctx context.Context, stateValue, code, oauthErr, oauthErrDesc string) (string, error) {
	if err := s.ensureEnabled(); err != nil {
		return "", err
	}

	state, err := s.Repo.Queries().GetStripeOAuthState(ctx, stateValue)
	if err != nil {
		return "", err
	}
	if state.UsedAt != nil {
		return "", fmt.Errorf("stripe oauth state already used")
	}
	if time.Now().UTC().After(state.ExpiresAt) {
		return "", fmt.Errorf("stripe oauth state expired")
	}
	if oauthErr != "" {
		return buildRedirectURL(s.Config.Stripe.Connect.ErrorURL, map[string]string{
			"provider":          "stripe",
			"workspace_id":      state.WorkspaceID,
			"error":             oauthErr,
			"error_description": oauthErrDesc,
		})
	}
	if strings.TrimSpace(code) == "" {
		return "", fmt.Errorf("stripe oauth code is required")
	}

	token, err := s.exchangeStripeAuthorizationCode(ctx, code)
	if err != nil {
		return buildRedirectURL(s.Config.Stripe.Connect.ErrorURL, map[string]string{
			"provider":          "stripe",
			"workspace_id":      state.WorkspaceID,
			"error":             "token_exchange_failed",
			"error_description": err.Error(),
		})
	}
	account, rawAccount, err := s.fetchStripeAccount(ctx, token.StripeUserID)
	if err != nil {
		return "", err
	}
	existing, err := s.Repo.Queries().GetStripeConnectionByAccountAndWorkspace(ctx, state.WorkspaceID, account.ID)
	if err != nil && err != store.ErrNotFound {
		return "", err
	}

	conn := domain.StripeConnection{
		ID:              uuid.New(),
		WorkspaceID:     state.WorkspaceID,
		StripeAccountID: account.ID,
		StripeUserID:    token.StripeUserID,
		Livemode:        token.Livemode,
		Scope:           token.Scope,
		RawAccount:      rawAccount,
		Status:          domain.ConnectionStatusActive,
		ConnectedAt:     time.Now().UTC(),
	}
	if existing.ID != uuid.Nil {
		conn = existing
		conn.StripeAccountID = account.ID
		conn.StripeUserID = token.StripeUserID
		conn.Livemode = token.Livemode
		conn.Scope = token.Scope
		conn.RawAccount = rawAccount
		conn.Status = domain.ConnectionStatusActive
		conn.ConnectedAt = time.Now().UTC()
		conn.DisconnectedAt = nil
	}
	if strings.TrimSpace(account.Email) != "" {
		conn.AccountEmail = &account.Email
	}
	name := strings.TrimSpace(account.BusinessProfile.Name)
	if name == "" {
		name = strings.TrimSpace(account.Settings.Dashboard.DisplayName)
	}
	if name != "" {
		conn.BusinessName = &name
	}
	if strings.TrimSpace(account.Country) != "" {
		conn.Country = &account.Country
	}

	if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
		if err := q.MarkStripeOAuthStateUsed(ctx, state.State); err != nil {
			return err
		}
		return q.SaveStripeConnection(ctx, conn)
	}); err != nil {
		return "", err
	}

	return buildRedirectURL(s.Config.Stripe.Connect.SuccessURL, map[string]string{
		"provider":         "stripe",
		"workspace_id":     state.WorkspaceID,
		"stripe_account_id": account.ID,
	})
}

func (s *Service) ListStripeConnections(ctx context.Context, workspaceID string) ([]domain.StripeConnection, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	return s.Repo.Queries().ListStripeConnectionsByWorkspace(ctx, workspaceID)
}

func (s *Service) DisconnectStripeConnection(ctx context.Context, workspaceID, accountID string) error {
	conn, err := s.Repo.Queries().GetStripeConnectionByAccountAndWorkspace(ctx, workspaceID, accountID)
	if err != nil {
		return err
	}
	if err := s.deauthorizeStripeUser(ctx, conn.StripeUserID); err != nil {
		return err
	}

	pairings, err := s.Repo.Queries().ListWorkspacePairingRecordsByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	return s.Repo.InTx(ctx, func(q *store.Queries) error {
		for _, pairing := range pairings {
			if pairing.StripeConnection.ID == conn.ID && pairing.Pairing.Status == domain.PairingStatusActive {
				if err := q.DisconnectWorkspacePairing(ctx, pairing.Pairing.ID); err != nil {
					return err
				}
			}
		}
		return q.DisconnectStripeConnection(ctx, conn.ID)
	})
}

func (s *Service) CreatePairing(ctx context.Context, workspaceID, stripeAccountID string, bokioCompanyID uuid.UUID) (domain.PairingRecord, error) {
	stripeConn, err := s.Repo.Queries().GetStripeConnectionByAccountAndWorkspace(ctx, workspaceID, stripeAccountID)
	if err != nil {
		return domain.PairingRecord{}, err
	}
	bokioConn, err := s.Repo.Queries().GetBokioConnectionByCompanyAndWorkspace(ctx, workspaceID, bokioCompanyID)
	if err != nil {
		return domain.PairingRecord{}, err
	}
	pairing := domain.WorkspacePairing{
		ID:                uuid.New(),
		WorkspaceID:       workspaceID,
		StripeConnectionID: stripeConn.ID,
		BokioConnectionID: bokioConn.ID,
		Status:            domain.PairingStatusActive,
		CreatedAt:         time.Now().UTC(),
	}
	if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
		return q.SaveWorkspacePairing(ctx, pairing)
	}); err != nil {
		return domain.PairingRecord{}, err
	}
	return s.Repo.Queries().GetWorkspacePairingRecord(ctx, pairing.ID)
}

func (s *Service) ListPairings(ctx context.Context, workspaceID string) ([]domain.PairingRecord, error) {
	return s.Repo.Queries().ListWorkspacePairingRecordsByWorkspace(ctx, workspaceID)
}

func (s *Service) GetPairing(ctx context.Context, workspaceID string, pairingID uuid.UUID) (domain.PairingRecord, error) {
	record, err := s.Repo.Queries().GetWorkspacePairingRecord(ctx, pairingID)
	if err != nil {
		return domain.PairingRecord{}, err
	}
	if record.Pairing.WorkspaceID != workspaceID {
		return domain.PairingRecord{}, store.ErrNotFound
	}
	return record, nil
}

func (s *Service) DisconnectPairing(ctx context.Context, workspaceID string, pairingID uuid.UUID) error {
	record, err := s.GetPairing(ctx, workspaceID, pairingID)
	if err != nil {
		return err
	}
	return s.Repo.InTx(ctx, func(q *store.Queries) error {
		return q.DisconnectWorkspacePairing(ctx, record.Pairing.ID)
	})
}

func (s *Service) ValidatePairing(ctx context.Context, workspaceID string, pairingID uuid.UUID) (map[string]any, error) {
	record, err := s.GetPairing(ctx, workspaceID, pairingID)
	if err != nil {
		return nil, err
	}
	runtime, err := s.runtimeFromRecord(ctx, record)
	if err != nil {
		return nil, err
	}
	account, _, err := s.fetchStripeAccount(ctx, runtime.StripeAccountID)
	if err != nil {
		return nil, err
	}
	bokioValidation, err := s.ValidateBokioConnection(ctx, workspaceID, runtime.BokioCompanyID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status": "ok",
		"pairing": map[string]any{
			"id":               record.Pairing.ID,
			"workspace_id":     record.Pairing.WorkspaceID,
			"stripe_account_id": runtime.StripeAccountID,
			"bokio_company_id": runtime.BokioCompanyID,
			"livemode":         runtime.Livemode,
		},
		"stripe": account,
		"bokio":  bokioValidation,
	}, nil
}

func (s *Service) exchangeStripeAuthorizationCode(ctx context.Context, code string) (stripeOAuthTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_secret", s.Config.Stripe.APIKey)

	var response stripeOAuthTokenResponse
	if err := doJSON(ctx, s.HTTPClient, http.MethodPost, s.Config.Stripe.Connect.TokenURL, nil, form, nil, &response); err != nil {
		return stripeOAuthTokenResponse{}, err
	}
	return response, nil
}

func (s *Service) deauthorizeStripeUser(ctx context.Context, stripeUserID string) error {
	form := url.Values{}
	form.Set("client_id", s.Config.Stripe.Connect.ClientID)
	form.Set("stripe_user_id", stripeUserID)
	return doJSON(ctx, s.HTTPClient, http.MethodPost, s.Config.Stripe.Connect.DeauthorizeURL, map[string]string{
		"Authorization": "Bearer " + s.Config.Stripe.APIKey,
	}, form, nil, nil)
}

func (s *Service) fetchStripeAccount(ctx context.Context, accountID string) (stripeAccountPayload, json.RawMessage, error) {
	endpoint, err := resolveURL(s.Config.Stripe.APIBaseURL, "/v1/account")
	if err != nil {
		return stripeAccountPayload{}, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return stripeAccountPayload{}, nil, fmt.Errorf("create stripe account request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.Config.Stripe.APIKey)
	req.Header.Set("Stripe-Account", accountID)
	req.Header.Set("Accept", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return stripeAccountPayload{}, nil, fmt.Errorf("call stripe account api: %w", err)
	}
	defer resp.Body.Close()

	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return stripeAccountPayload{}, nil, fmt.Errorf("decode stripe account response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return stripeAccountPayload{}, nil, fmt.Errorf("stripe account api returned %d", resp.StatusCode)
	}
	var payload stripeAccountPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return stripeAccountPayload{}, nil, fmt.Errorf("decode stripe account payload: %w", err)
	}
	return payload, raw, nil
}

func buildRedirectURL(base string, queryValues map[string]string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse redirect url: %w", err)
	}
	query := u.Query()
	for key, value := range queryValues {
		if strings.TrimSpace(value) == "" {
			continue
		}
		query.Set(key, value)
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}
