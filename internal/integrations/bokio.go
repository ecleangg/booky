package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ecleangg/booky/internal/bokio"
	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/store"
	"github.com/google/uuid"
)

type bokioTokenResponse struct {
	TenantID     uuid.UUID `json:"tenant_id"`
	TenantType   string    `json:"tenant_type"`
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	RefreshToken string    `json:"refresh_token"`
	Scope        string    `json:"scope"`
}

type bokioConnectionPayload struct {
	ID         uuid.UUID `json:"id"`
	TenantID   uuid.UUID `json:"tenantId"`
	TenantName string    `json:"tenantName"`
	Type       string    `json:"type"`
}

func (s *Service) StartBokioOAuth(ctx context.Context, workspaceID, subject string, requestedTenantID *uuid.UUID) (string, time.Time, error) {
	if err := s.ensureEnabled(); err != nil {
		return "", time.Time{}, err
	}

	state := domain.BokioOAuthState{
		State:       uuid.NewString(),
		WorkspaceID: workspaceID,
		UserSubject: subject,
		ExpiresAt:   time.Now().UTC().Add(10 * time.Minute),
	}
	if requestedTenantID != nil {
		state.RequestedTenantID = requestedTenantID
		tenantType := "company"
		state.RequestedTenantType = &tenantType
	}
	if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
		return q.CreateBokioOAuthState(ctx, state)
	}); err != nil {
		return "", time.Time{}, err
	}

	u, err := url.Parse(s.Config.Bokio.OAuth.AuthorizeURL)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("parse bokio authorize url: %w", err)
	}
	query := u.Query()
	query.Set("response_type", "code")
	query.Set("client_id", s.Config.Bokio.OAuth.ClientID)
	query.Set("redirect_uri", s.Config.Bokio.OAuth.RedirectURI)
	query.Set("scope", s.Config.Bokio.OAuth.Scope)
	query.Set("state", state.State)
	if requestedTenantID != nil {
		query.Set("bokio_tenantid", requestedTenantID.String())
		query.Set("bokio_tenanttype", "company")
	}
	u.RawQuery = query.Encode()
	return u.String(), state.ExpiresAt, nil
}

func (s *Service) CompleteBokioOAuth(ctx context.Context, stateValue, code, oauthErr, oauthErrDesc string) (string, error) {
	if err := s.ensureEnabled(); err != nil {
		return "", err
	}

	state, err := s.Repo.Queries().GetBokioOAuthState(ctx, stateValue)
	if err != nil {
		return "", err
	}
	if state.UsedAt != nil {
		return "", fmt.Errorf("bokio oauth state already used")
	}
	if time.Now().UTC().After(state.ExpiresAt) {
		return "", fmt.Errorf("bokio oauth state expired")
	}
	if oauthErr != "" {
		return buildRedirectURL(s.Config.Bokio.OAuth.ErrorURL, map[string]string{
			"provider":          "bokio",
			"workspace_id":      state.WorkspaceID,
			"error":             oauthErr,
			"error_description": oauthErrDesc,
		})
	}
	if strings.TrimSpace(code) == "" {
		return "", fmt.Errorf("bokio oauth code is required")
	}

	token, err := s.exchangeBokioAuthorizationCode(ctx, code)
	if err != nil {
		return buildRedirectURL(s.Config.Bokio.OAuth.ErrorURL, map[string]string{
			"provider":          "bokio",
			"workspace_id":      state.WorkspaceID,
			"error":             "token_exchange_failed",
			"error_description": err.Error(),
		})
	}
	generalToken, err := s.bokioClientCredentialsToken(ctx)
	if err != nil {
		return "", err
	}
	connectionPayload, err := s.findBokioConnection(ctx, generalToken, token.TenantID)
	if err != nil {
		return "", err
	}
	companyName, err := s.fetchBokioCompanyName(ctx, token.TenantID, token.AccessToken)
	if err != nil {
		return "", err
	}
	existing, err := s.Repo.Queries().GetBokioConnectionByCompanyAndWorkspace(ctx, state.WorkspaceID, token.TenantID)
	if err != nil && err != store.ErrNotFound {
		return "", err
	}
	conn := domain.BokioConnection{
		ID:                 uuid.New(),
		WorkspaceID:        state.WorkspaceID,
		BokioConnectionID:  connectionPayload.ID,
		BokioCompanyID:     token.TenantID,
		CompanyName:        companyName,
		TokenExpiresAt:     time.Now().UTC().Add(time.Duration(token.ExpiresIn) * time.Second),
		Scope:              token.Scope,
		Settings:           nil,
		SettingsVersion:    1,
		Status:             domain.ConnectionStatusActive,
		ConnectedAt:        time.Now().UTC(),
	}
	if existing.ID != uuid.Nil {
		conn = existing
		conn.BokioConnectionID = connectionPayload.ID
		conn.CompanyName = companyName
		conn.TokenExpiresAt = time.Now().UTC().Add(time.Duration(token.ExpiresIn) * time.Second)
		conn.Scope = token.Scope
		conn.Status = domain.ConnectionStatusActive
		conn.ConnectedAt = time.Now().UTC()
		conn.DisconnectedAt = nil
	}
	conn.AccessTokenCipher, err = s.Cipher.Encrypt(token.AccessToken)
	if err != nil {
		return "", err
	}
	conn.RefreshTokenCipher, err = s.Cipher.Encrypt(token.RefreshToken)
	if err != nil {
		return "", err
	}

	if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
		if err := q.MarkBokioOAuthStateUsed(ctx, state.State); err != nil {
			return err
		}
		return q.SaveBokioConnection(ctx, conn)
	}); err != nil {
		return "", err
	}

	return buildRedirectURL(s.Config.Bokio.OAuth.SuccessURL, map[string]string{
		"provider":         "bokio",
		"workspace_id":     state.WorkspaceID,
		"bokio_company_id": token.TenantID.String(),
	})
}

func (s *Service) ListBokioConnections(ctx context.Context, workspaceID string) ([]domain.BokioConnection, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	return s.Repo.Queries().ListBokioConnectionsByWorkspace(ctx, workspaceID)
}

func (s *Service) GetBokioConnection(ctx context.Context, workspaceID string, companyID uuid.UUID) (domain.BokioConnection, CompanySettings, error) {
	conn, err := s.Repo.Queries().GetBokioConnectionByCompanyAndWorkspace(ctx, workspaceID, companyID)
	if err != nil {
		return domain.BokioConnection{}, CompanySettings{}, err
	}
	settings, err := s.companySettingsForConnection(conn)
	if err != nil {
		return domain.BokioConnection{}, CompanySettings{}, err
	}
	return conn, settings, nil
}

func (s *Service) UpdateBokioCompanySettings(ctx context.Context, workspaceID string, companyID uuid.UUID, settings CompanySettings) (domain.BokioConnection, error) {
	if err := ValidateCompanySettings(settings); err != nil {
		return domain.BokioConnection{}, err
	}
	conn, err := s.Repo.Queries().GetBokioConnectionByCompanyAndWorkspace(ctx, workspaceID, companyID)
	if err != nil {
		return domain.BokioConnection{}, err
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		return domain.BokioConnection{}, fmt.Errorf("encode company settings: %w", err)
	}
	conn.Settings = raw
	conn.SettingsVersion++
	if conn.SettingsVersion == 0 {
		conn.SettingsVersion = 1
	}
	if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
		return q.SaveBokioConnection(ctx, conn)
	}); err != nil {
		return domain.BokioConnection{}, err
	}
	return conn, nil
}

func (s *Service) GetBokioChartOfAccounts(ctx context.Context, workspaceID string, companyID uuid.UUID) ([]bokio.ChartAccount, error) {
	conn, err := s.Repo.Queries().GetBokioConnectionByCompanyAndWorkspace(ctx, workspaceID, companyID)
	if err != nil {
		return nil, err
	}
	conn, accessToken, err := s.ensureBokioAccessToken(ctx, conn)
	if err != nil {
		return nil, err
	}
	_ = conn
	client := bokio.NewClient(config.BokioConfig{
		CompanyID: companyID,
		Token:     accessToken,
		BaseURL:   s.Config.Bokio.BaseURL,
	})
	return client.GetChartOfAccounts(ctx)
}

func (s *Service) ValidateBokioConnection(ctx context.Context, workspaceID string, companyID uuid.UUID) (map[string]any, error) {
	conn, err := s.Repo.Queries().GetBokioConnectionByCompanyAndWorkspace(ctx, workspaceID, companyID)
	if err != nil {
		return nil, err
	}
	settings, err := s.companySettingsForConnection(conn)
	if err != nil {
		return nil, err
	}
	conn, accessToken, err := s.ensureBokioAccessToken(ctx, conn)
	if err != nil {
		return nil, err
	}
	client := bokio.NewClient(config.BokioConfig{
		CompanyID: conn.BokioCompanyID,
		Token:     accessToken,
		BaseURL:   s.Config.Bokio.BaseURL,
	})
	checkResult, err := client.Check(ctx)
	if err != nil {
		return nil, err
	}
	accountValidationErr := client.ValidateAccounts(ctx, collectConfiguredAccounts(settings.Accounts))
	return map[string]any{
		"status":                "ok",
		"bokio":                 checkResult,
		"settings_valid":        accountValidationErr == nil,
		"settings_error":        errorString(accountValidationErr),
		"settings_version":      conn.SettingsVersion,
		"oss_reporting_enabled": settings.OSSReportingEnabled,
	}, nil
}

func (s *Service) DisconnectBokioConnection(ctx context.Context, workspaceID string, companyID uuid.UUID) error {
	conn, err := s.Repo.Queries().GetBokioConnectionByCompanyAndWorkspace(ctx, workspaceID, companyID)
	if err != nil {
		return err
	}
	generalToken, err := s.bokioClientCredentialsToken(ctx)
	if err != nil {
		return err
	}
	if err := s.deleteBokioConnection(ctx, generalToken, conn.BokioConnectionID); err != nil {
		return err
	}

	pairings, err := s.Repo.Queries().ListWorkspacePairingRecordsByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	return s.Repo.InTx(ctx, func(q *store.Queries) error {
		for _, pairing := range pairings {
			if pairing.BokioConnection.ID == conn.ID && pairing.Pairing.Status == domain.PairingStatusActive {
				if err := q.DisconnectWorkspacePairing(ctx, pairing.Pairing.ID); err != nil {
					return err
				}
			}
		}
		return q.DisconnectBokioConnection(ctx, conn.ID)
	})
}

func (s *Service) ensureBokioAccessToken(ctx context.Context, conn domain.BokioConnection) (domain.BokioConnection, string, error) {
	accessToken, err := s.Cipher.Decrypt(conn.AccessTokenCipher)
	if err != nil {
		return domain.BokioConnection{}, "", err
	}
	if time.Now().UTC().Before(conn.TokenExpiresAt.Add(-1 * time.Minute)) {
		return conn, accessToken, nil
	}

	refreshToken, err := s.Cipher.Decrypt(conn.RefreshTokenCipher)
	if err != nil {
		return domain.BokioConnection{}, "", err
	}
	refreshed, err := s.refreshBokioToken(ctx, refreshToken)
	if err != nil {
		return domain.BokioConnection{}, "", err
	}
	conn.TokenExpiresAt = time.Now().UTC().Add(time.Duration(refreshed.ExpiresIn) * time.Second)
	conn.Scope = refreshed.Scope
	conn.Status = domain.ConnectionStatusActive
	conn.DisconnectedAt = nil
	conn.AccessTokenCipher, err = s.Cipher.Encrypt(refreshed.AccessToken)
	if err != nil {
		return domain.BokioConnection{}, "", err
	}
	if strings.TrimSpace(refreshed.RefreshToken) != "" {
		conn.RefreshTokenCipher, err = s.Cipher.Encrypt(refreshed.RefreshToken)
		if err != nil {
			return domain.BokioConnection{}, "", err
		}
	}
	if err := s.Repo.InTx(ctx, func(q *store.Queries) error {
		return q.SaveBokioConnection(ctx, conn)
	}); err != nil {
		return domain.BokioConnection{}, "", err
	}
	return conn, refreshed.AccessToken, nil
}

func (s *Service) exchangeBokioAuthorizationCode(ctx context.Context, code string) (bokioTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", s.Config.Bokio.OAuth.RedirectURI)

	var response bokioTokenResponse
	if err := doJSON(ctx, s.HTTPClient, http.MethodPost, s.Config.Bokio.OAuth.TokenURL, map[string]string{
		"Authorization": "Bearer " + s.Config.Bokio.OAuth.ClientSecret,
	}, form, nil, &response); err != nil {
		return bokioTokenResponse{}, err
	}
	return response, nil
}

func (s *Service) refreshBokioToken(ctx context.Context, refreshToken string) (bokioTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)

	var response bokioTokenResponse
	if err := doJSON(ctx, s.HTTPClient, http.MethodPost, s.Config.Bokio.OAuth.TokenURL, map[string]string{
		"Authorization": "Bearer " + s.Config.Bokio.OAuth.ClientSecret,
	}, form, nil, &response); err != nil {
		return bokioTokenResponse{}, err
	}
	return response, nil
}

func (s *Service) bokioClientCredentialsToken(ctx context.Context) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	var response bokioTokenResponse
	if err := doJSON(ctx, s.HTTPClient, http.MethodPost, s.Config.Bokio.OAuth.TokenURL, map[string]string{
		"Authorization": "Bearer " + s.Config.Bokio.OAuth.ClientSecret,
	}, form, nil, &response); err != nil {
		return "", err
	}
	return response.AccessToken, nil
}

func (s *Service) findBokioConnection(ctx context.Context, generalAccessToken string, companyID uuid.UUID) (bokioConnectionPayload, error) {
	endpoint, err := resolveURL(s.Config.Bokio.OAuth.GeneralBaseURL, "/connections?tenantId="+url.QueryEscape(companyID.String()))
	if err != nil {
		return bokioConnectionPayload{}, err
	}
	var response struct {
		Items []bokioConnectionPayload `json:"items"`
	}
	if err := doJSON(ctx, s.HTTPClient, http.MethodGet, endpoint, map[string]string{
		"Authorization": "Bearer " + generalAccessToken,
	}, nil, nil, &response); err != nil {
		return bokioConnectionPayload{}, err
	}
	for _, item := range response.Items {
		if item.TenantID == companyID {
			return item, nil
		}
	}
	return bokioConnectionPayload{}, fmt.Errorf("bokio connection not found for company %s", companyID)
}

func (s *Service) deleteBokioConnection(ctx context.Context, generalAccessToken string, connectionID uuid.UUID) error {
	endpoint, err := resolveURL(s.Config.Bokio.OAuth.GeneralBaseURL, "/connections/"+connectionID.String())
	if err != nil {
		return err
	}
	return doJSON(ctx, s.HTTPClient, http.MethodDelete, endpoint, map[string]string{
		"Authorization": "Bearer " + generalAccessToken,
	}, nil, nil, nil)
}

func (s *Service) fetchBokioCompanyName(ctx context.Context, companyID uuid.UUID, accessToken string) (string, error) {
	client := bokio.NewClient(config.BokioConfig{
		CompanyID: companyID,
		Token:     accessToken,
		BaseURL:   s.Config.Bokio.BaseURL,
	})
	info, err := client.GetCompanyInformation(ctx)
	if err != nil {
		return "", err
	}
	return info.Name, nil
}

func collectConfiguredAccounts(accounts config.AccountsConfig) []int {
	values := []int{
		accounts.StripeReceivable,
		accounts.Bank,
		accounts.Dispute,
		accounts.FallbackOBS,
		accounts.Rounding,
		accounts.StripeFees.Expense,
		accounts.StripeFees.InputVAT,
		accounts.StripeFees.OutputVAT,
	}
	for _, account := range accounts.StripeBalanceByCurrency {
		values = append(values, account)
	}
	for _, market := range accounts.SalesByMarket {
		values = append(values, market.Revenue)
		if market.OutputVAT != 0 {
			values = append(values, market.OutputVAT)
		}
	}
	if accounts.OtherCountriesDefault != nil {
		values = append(values, accounts.OtherCountriesDefault.Revenue)
		if accounts.OtherCountriesDefault.OutputVAT != 0 {
			values = append(values, accounts.OtherCountriesDefault.OutputVAT)
		}
	}
	seen := map[int]struct{}{}
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func errorString(err error) *string {
	if err == nil {
		return nil
	}
	value := err.Error()
	return &value
}
