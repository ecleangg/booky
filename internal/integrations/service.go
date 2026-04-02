package integrations

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/domain"
	"github.com/ecleangg/booky/internal/secure"
	"github.com/ecleangg/booky/internal/store"
	"github.com/google/uuid"
)

type Service struct {
	Config     config.Config
	Repo       *store.Repository
	Cipher     *secure.Cipher
	HTTPClient *http.Client
	Logger     *slog.Logger
}

func NewService(cfg config.Config, repo *store.Repository, logger *slog.Logger) (*Service, error) {
	if logger == nil {
		logger = slog.Default()
	}

	var cipherInstance *secure.Cipher
	var err error
	if cfg.WorkspacePairingsEnabled() {
		cipherInstance, err = secure.NewCipher(cfg.Security.DataEncryptionKey)
		if err != nil {
			return nil, err
		}
	}

	return &Service{
		Config:     cfg,
		Repo:       repo,
		Cipher:     cipherInstance,
		HTTPClient: &http.Client{Timeout: defaultHTTPTimeout},
		Logger:     logger,
	}, nil
}

func (s *Service) Enabled() bool {
	return s != nil && s.Config.WorkspacePairingsEnabled()
}

func (s *Service) ResolveRuntimeByStripeAccount(ctx context.Context, accountID string, livemode bool) (RuntimeConfig, error) {
	record, err := s.Repo.Queries().GetActivePairingRecordByStripeAccount(ctx, accountID, livemode)
	if err != nil {
		return RuntimeConfig{}, err
	}
	return s.runtimeFromRecord(ctx, record)
}

func (s *Service) ResolveRuntimeByCompanyID(ctx context.Context, companyID uuid.UUID) (RuntimeConfig, error) {
	record, err := s.Repo.Queries().GetActivePairingRecordByCompanyID(ctx, companyID)
	if err != nil {
		return RuntimeConfig{}, err
	}
	return s.runtimeFromRecord(ctx, record)
}

func (s *Service) ListActiveRuntimes(ctx context.Context) ([]RuntimeConfig, error) {
	records, err := s.Repo.Queries().ListActivePairingRecords(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RuntimeConfig, 0, len(records))
	for _, record := range records {
		runtime, err := s.runtimeFromRecord(ctx, record)
		if err != nil {
			return nil, err
		}
		out = append(out, runtime)
	}
	return out, nil
}

func (s *Service) runtimeFromRecord(ctx context.Context, record domain.PairingRecord) (RuntimeConfig, error) {
	conn, token, err := s.ensureBokioAccessToken(ctx, record.BokioConnection)
	if err != nil {
		return RuntimeConfig{}, err
	}
	settings, err := ParseCompanySettings(conn.Settings, DefaultCompanySettings(s.Config))
	if err != nil {
		return RuntimeConfig{}, err
	}
	return RuntimeConfig{
		WorkspaceID:      record.Pairing.WorkspaceID,
		PairingID:        record.Pairing.ID,
		StripeAccountID:  record.StripeConnection.StripeAccountID,
		Livemode:         record.StripeConnection.Livemode,
		BokioCompanyID:   record.BokioConnection.BokioCompanyID,
		BokioCompanyName: record.BokioConnection.CompanyName,
		BokioToken:       token,
		Settings:         settings,
	}, nil
}

func (s *Service) companySettingsForConnection(conn domain.BokioConnection) (CompanySettings, error) {
	return ParseCompanySettings(conn.Settings, DefaultCompanySettings(s.Config))
}

func (s *Service) ensureEnabled() error {
	if !s.Enabled() {
		return fmt.Errorf("workspace pairings are not configured")
	}
	return nil
}
