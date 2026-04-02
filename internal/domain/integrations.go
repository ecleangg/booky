package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	ConnectionStatusActive       = "active"
	ConnectionStatusDisconnected = "disconnected"
	PairingStatusActive          = "active"
	PairingStatusDisconnected    = "disconnected"
)

type StripeOAuthState struct {
	State       string
	WorkspaceID string
	UserSubject string
	ExpiresAt   time.Time
	UsedAt      *time.Time
	CreatedAt   time.Time
}

type StripeConnection struct {
	ID             uuid.UUID
	WorkspaceID    string
	StripeAccountID string
	StripeUserID   string
	Livemode       bool
	Scope          string
	AccountEmail   *string
	BusinessName   *string
	Country        *string
	RawAccount     json.RawMessage
	Status         string
	ConnectedAt    time.Time
	DisconnectedAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type BokioOAuthState struct {
	State              string
	WorkspaceID        string
	UserSubject        string
	RequestedTenantID  *uuid.UUID
	RequestedTenantType *string
	ExpiresAt          time.Time
	UsedAt             *time.Time
	CreatedAt          time.Time
}

type BokioConnection struct {
	ID                 uuid.UUID
	WorkspaceID        string
	BokioConnectionID  uuid.UUID
	BokioCompanyID     uuid.UUID
	CompanyName        string
	AccessTokenCipher  string
	RefreshTokenCipher string
	TokenExpiresAt     time.Time
	Scope              string
	Settings           json.RawMessage
	SettingsVersion    int
	Status             string
	ConnectedAt        time.Time
	DisconnectedAt     *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type WorkspacePairing struct {
	ID                uuid.UUID
	WorkspaceID       string
	StripeConnectionID uuid.UUID
	BokioConnectionID uuid.UUID
	Status            string
	CreatedAt         time.Time
	DisconnectedAt    *time.Time
	UpdatedAt         time.Time
}

type PairingRecord struct {
	Pairing         WorkspacePairing
	StripeConnection StripeConnection
	BokioConnection BokioConnection
}
