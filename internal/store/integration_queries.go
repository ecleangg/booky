package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ecleangg/booky/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (q *Queries) CreateStripeOAuthState(ctx context.Context, state domain.StripeOAuthState) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO stripe_oauth_states (state, workspace_id, user_subject, expires_at, used_at, created_at)
		VALUES ($1, $2, $3, $4, $5, COALESCE($6, now()))
	`, state.State, state.WorkspaceID, state.UserSubject, state.ExpiresAt, state.UsedAt, nullableTime(state.CreatedAt))
	if err != nil {
		return fmt.Errorf("create stripe oauth state: %w", err)
	}
	return nil
}

func (q *Queries) GetStripeOAuthState(ctx context.Context, state string) (domain.StripeOAuthState, error) {
	row := q.db.QueryRow(ctx, `
		SELECT state, workspace_id, user_subject, expires_at, used_at, created_at
		FROM stripe_oauth_states
		WHERE state = $1
	`, state)
	return scanStripeOAuthState(row)
}

func (q *Queries) MarkStripeOAuthStateUsed(ctx context.Context, state string) error {
	_, err := q.db.Exec(ctx, `UPDATE stripe_oauth_states SET used_at = now() WHERE state = $1`, state)
	if err != nil {
		return fmt.Errorf("mark stripe oauth state used: %w", err)
	}
	return nil
}

func (q *Queries) CreateBokioOAuthState(ctx context.Context, state domain.BokioOAuthState) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO bokio_oauth_states (
			state, workspace_id, user_subject, requested_tenant_id, requested_tenant_type,
			expires_at, used_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE($8, now()))
	`, state.State, state.WorkspaceID, state.UserSubject, state.RequestedTenantID, state.RequestedTenantType, state.ExpiresAt, state.UsedAt, nullableTime(state.CreatedAt))
	if err != nil {
		return fmt.Errorf("create bokio oauth state: %w", err)
	}
	return nil
}

func (q *Queries) GetBokioOAuthState(ctx context.Context, state string) (domain.BokioOAuthState, error) {
	row := q.db.QueryRow(ctx, `
		SELECT state, workspace_id, user_subject, requested_tenant_id, requested_tenant_type, expires_at, used_at, created_at
		FROM bokio_oauth_states
		WHERE state = $1
	`, state)
	return scanBokioOAuthState(row)
}

func (q *Queries) MarkBokioOAuthStateUsed(ctx context.Context, state string) error {
	_, err := q.db.Exec(ctx, `UPDATE bokio_oauth_states SET used_at = now() WHERE state = $1`, state)
	if err != nil {
		return fmt.Errorf("mark bokio oauth state used: %w", err)
	}
	return nil
}

func (q *Queries) SaveStripeConnection(ctx context.Context, conn domain.StripeConnection) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO stripe_connections (
			id, workspace_id, stripe_account_id, stripe_user_id, livemode, scope,
			account_email, business_name, country, raw_account, status,
			connected_at, disconnected_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, COALESCE($14, now()), now()
		)
		ON CONFLICT (id) DO UPDATE SET
			workspace_id = EXCLUDED.workspace_id,
			stripe_account_id = EXCLUDED.stripe_account_id,
			stripe_user_id = EXCLUDED.stripe_user_id,
			livemode = EXCLUDED.livemode,
			scope = EXCLUDED.scope,
			account_email = EXCLUDED.account_email,
			business_name = EXCLUDED.business_name,
			country = EXCLUDED.country,
			raw_account = EXCLUDED.raw_account,
			status = EXCLUDED.status,
			connected_at = EXCLUDED.connected_at,
			disconnected_at = EXCLUDED.disconnected_at,
			updated_at = now()
	`, conn.ID, conn.WorkspaceID, conn.StripeAccountID, conn.StripeUserID, conn.Livemode, conn.Scope,
		conn.AccountEmail, conn.BusinessName, conn.Country, []byte(conn.RawAccount), conn.Status,
		conn.ConnectedAt, conn.DisconnectedAt, nullableTime(conn.CreatedAt))
	if err != nil {
		return fmt.Errorf("save stripe connection: %w", err)
	}
	return nil
}

func (q *Queries) GetLatestStripeConnectionByAccount(ctx context.Context, accountID string, livemode bool) (domain.StripeConnection, error) {
	row := q.db.QueryRow(ctx, `
		SELECT id, workspace_id, stripe_account_id, stripe_user_id, livemode, scope, account_email,
			business_name, country, raw_account, status, connected_at, disconnected_at, created_at, updated_at
		FROM stripe_connections
		WHERE stripe_account_id = $1 AND livemode = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, accountID, livemode)
	return scanStripeConnection(row)
}

func (q *Queries) GetStripeConnectionByAccountAndWorkspace(ctx context.Context, workspaceID, accountID string) (domain.StripeConnection, error) {
	row := q.db.QueryRow(ctx, `
		SELECT id, workspace_id, stripe_account_id, stripe_user_id, livemode, scope, account_email,
			business_name, country, raw_account, status, connected_at, disconnected_at, created_at, updated_at
		FROM stripe_connections
		WHERE workspace_id = $1 AND stripe_account_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, workspaceID, accountID)
	return scanStripeConnection(row)
}

func (q *Queries) GetStripeConnectionByID(ctx context.Context, id uuid.UUID) (domain.StripeConnection, error) {
	row := q.db.QueryRow(ctx, `
		SELECT id, workspace_id, stripe_account_id, stripe_user_id, livemode, scope, account_email,
			business_name, country, raw_account, status, connected_at, disconnected_at, created_at, updated_at
		FROM stripe_connections
		WHERE id = $1
	`, id)
	return scanStripeConnection(row)
}

func (q *Queries) ListStripeConnectionsByWorkspace(ctx context.Context, workspaceID string) ([]domain.StripeConnection, error) {
	rows, err := q.db.Query(ctx, `
		SELECT id, workspace_id, stripe_account_id, stripe_user_id, livemode, scope, account_email,
			business_name, country, raw_account, status, connected_at, disconnected_at, created_at, updated_at
		FROM stripe_connections
		WHERE workspace_id = $1
		ORDER BY created_at DESC
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list stripe connections: %w", err)
	}
	defer rows.Close()

	var out []domain.StripeConnection
	for rows.Next() {
		record, err := scanStripeConnection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stripe connections: %w", err)
	}
	return out, nil
}

func (q *Queries) DisconnectStripeConnection(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.Exec(ctx, `
		UPDATE stripe_connections
		SET status = 'disconnected', disconnected_at = now(), updated_at = now()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("disconnect stripe connection: %w", err)
	}
	return nil
}

func (q *Queries) SaveBokioConnection(ctx context.Context, conn domain.BokioConnection) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO bokio_connections (
			id, workspace_id, bokio_connection_id, bokio_company_id, company_name,
			access_token_cipher, refresh_token_cipher, token_expires_at, scope, settings,
			settings_version, status, connected_at, disconnected_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14, COALESCE($15, now()), now()
		)
		ON CONFLICT (id) DO UPDATE SET
			workspace_id = EXCLUDED.workspace_id,
			bokio_connection_id = EXCLUDED.bokio_connection_id,
			bokio_company_id = EXCLUDED.bokio_company_id,
			company_name = EXCLUDED.company_name,
			access_token_cipher = EXCLUDED.access_token_cipher,
			refresh_token_cipher = EXCLUDED.refresh_token_cipher,
			token_expires_at = EXCLUDED.token_expires_at,
			scope = EXCLUDED.scope,
			settings = EXCLUDED.settings,
			settings_version = EXCLUDED.settings_version,
			status = EXCLUDED.status,
			connected_at = EXCLUDED.connected_at,
			disconnected_at = EXCLUDED.disconnected_at,
			updated_at = now()
	`, conn.ID, conn.WorkspaceID, conn.BokioConnectionID, conn.BokioCompanyID, conn.CompanyName,
		conn.AccessTokenCipher, conn.RefreshTokenCipher, conn.TokenExpiresAt, conn.Scope, []byte(conn.Settings),
		conn.SettingsVersion, conn.Status, conn.ConnectedAt, conn.DisconnectedAt, nullableTime(conn.CreatedAt))
	if err != nil {
		return fmt.Errorf("save bokio connection: %w", err)
	}
	return nil
}

func (q *Queries) GetLatestBokioConnectionByCompany(ctx context.Context, companyID uuid.UUID) (domain.BokioConnection, error) {
	row := q.db.QueryRow(ctx, `
		SELECT id, workspace_id, bokio_connection_id, bokio_company_id, company_name,
			access_token_cipher, refresh_token_cipher, token_expires_at, scope, settings,
			settings_version, status, connected_at, disconnected_at, created_at, updated_at
		FROM bokio_connections
		WHERE bokio_company_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, companyID)
	return scanBokioConnection(row)
}

func (q *Queries) GetBokioConnectionByCompanyAndWorkspace(ctx context.Context, workspaceID string, companyID uuid.UUID) (domain.BokioConnection, error) {
	row := q.db.QueryRow(ctx, `
		SELECT id, workspace_id, bokio_connection_id, bokio_company_id, company_name,
			access_token_cipher, refresh_token_cipher, token_expires_at, scope, settings,
			settings_version, status, connected_at, disconnected_at, created_at, updated_at
		FROM bokio_connections
		WHERE workspace_id = $1 AND bokio_company_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, workspaceID, companyID)
	return scanBokioConnection(row)
}

func (q *Queries) GetBokioConnectionByID(ctx context.Context, id uuid.UUID) (domain.BokioConnection, error) {
	row := q.db.QueryRow(ctx, `
		SELECT id, workspace_id, bokio_connection_id, bokio_company_id, company_name,
			access_token_cipher, refresh_token_cipher, token_expires_at, scope, settings,
			settings_version, status, connected_at, disconnected_at, created_at, updated_at
		FROM bokio_connections
		WHERE id = $1
	`, id)
	return scanBokioConnection(row)
}

func (q *Queries) ListBokioConnectionsByWorkspace(ctx context.Context, workspaceID string) ([]domain.BokioConnection, error) {
	rows, err := q.db.Query(ctx, `
		SELECT id, workspace_id, bokio_connection_id, bokio_company_id, company_name,
			access_token_cipher, refresh_token_cipher, token_expires_at, scope, settings,
			settings_version, status, connected_at, disconnected_at, created_at, updated_at
		FROM bokio_connections
		WHERE workspace_id = $1
		ORDER BY created_at DESC
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list bokio connections: %w", err)
	}
	defer rows.Close()

	var out []domain.BokioConnection
	for rows.Next() {
		record, err := scanBokioConnection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bokio connections: %w", err)
	}
	return out, nil
}

func (q *Queries) DisconnectBokioConnection(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.Exec(ctx, `
		UPDATE bokio_connections
		SET status = 'disconnected', disconnected_at = now(), updated_at = now()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("disconnect bokio connection: %w", err)
	}
	return nil
}

func (q *Queries) SaveWorkspacePairing(ctx context.Context, pairing domain.WorkspacePairing) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO workspace_pairings (
			id, workspace_id, stripe_connection_id, bokio_connection_id,
			status, created_at, disconnected_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, COALESCE($6, now()), $7, now())
		ON CONFLICT (id) DO UPDATE SET
			workspace_id = EXCLUDED.workspace_id,
			stripe_connection_id = EXCLUDED.stripe_connection_id,
			bokio_connection_id = EXCLUDED.bokio_connection_id,
			status = EXCLUDED.status,
			disconnected_at = EXCLUDED.disconnected_at,
			updated_at = now()
	`, pairing.ID, pairing.WorkspaceID, pairing.StripeConnectionID, pairing.BokioConnectionID, pairing.Status, nullableTime(pairing.CreatedAt), pairing.DisconnectedAt)
	if err != nil {
		return fmt.Errorf("save workspace pairing: %w", err)
	}
	return nil
}

func (q *Queries) GetWorkspacePairingRecord(ctx context.Context, id uuid.UUID) (domain.PairingRecord, error) {
	row := q.db.QueryRow(ctx, pairingRecordSelectSQL(`
		WHERE p.id = $1
	`), id)
	return scanPairingRecord(row)
}

func (q *Queries) ListWorkspacePairingRecordsByWorkspace(ctx context.Context, workspaceID string) ([]domain.PairingRecord, error) {
	rows, err := q.db.Query(ctx, pairingRecordSelectSQL(`
		WHERE p.workspace_id = $1
		ORDER BY p.created_at DESC
	`), workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list workspace pairings: %w", err)
	}
	defer rows.Close()

	var out []domain.PairingRecord
	for rows.Next() {
		record, err := scanPairingRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspace pairings: %w", err)
	}
	return out, nil
}

func (q *Queries) GetActivePairingRecordByStripeAccount(ctx context.Context, accountID string, livemode bool) (domain.PairingRecord, error) {
	row := q.db.QueryRow(ctx, pairingRecordSelectSQL(`
		WHERE p.status = 'active'
		  AND sc.status = 'active'
		  AND bc.status = 'active'
		  AND sc.stripe_account_id = $1
		  AND sc.livemode = $2
		ORDER BY p.created_at DESC
		LIMIT 1
	`), accountID, livemode)
	return scanPairingRecord(row)
}

func (q *Queries) GetActivePairingRecordByCompanyID(ctx context.Context, companyID uuid.UUID) (domain.PairingRecord, error) {
	row := q.db.QueryRow(ctx, pairingRecordSelectSQL(`
		WHERE p.status = 'active'
		  AND sc.status = 'active'
		  AND bc.status = 'active'
		  AND bc.bokio_company_id = $1
		ORDER BY p.created_at DESC
		LIMIT 1
	`), companyID)
	return scanPairingRecord(row)
}

func (q *Queries) ListActivePairingRecords(ctx context.Context) ([]domain.PairingRecord, error) {
	rows, err := q.db.Query(ctx, pairingRecordSelectSQL(`
		WHERE p.status = 'active'
		  AND sc.status = 'active'
		  AND bc.status = 'active'
		ORDER BY p.created_at DESC
	`))
	if err != nil {
		return nil, fmt.Errorf("list active pairings: %w", err)
	}
	defer rows.Close()

	var out []domain.PairingRecord
	for rows.Next() {
		record, err := scanPairingRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active pairings: %w", err)
	}
	return out, nil
}

func (q *Queries) DisconnectWorkspacePairing(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.Exec(ctx, `
		UPDATE workspace_pairings
		SET status = 'disconnected', disconnected_at = now(), updated_at = now()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("disconnect workspace pairing: %w", err)
	}
	return nil
}

func pairingRecordSelectSQL(suffix string) string {
	return `
		SELECT
			p.id, p.workspace_id, p.stripe_connection_id, p.bokio_connection_id, p.status, p.created_at, p.disconnected_at, p.updated_at,
			sc.id, sc.workspace_id, sc.stripe_account_id, sc.stripe_user_id, sc.livemode, sc.scope, sc.account_email, sc.business_name, sc.country, sc.raw_account, sc.status, sc.connected_at, sc.disconnected_at, sc.created_at, sc.updated_at,
			bc.id, bc.workspace_id, bc.bokio_connection_id, bc.bokio_company_id, bc.company_name, bc.access_token_cipher, bc.refresh_token_cipher, bc.token_expires_at, bc.scope, bc.settings, bc.settings_version, bc.status, bc.connected_at, bc.disconnected_at, bc.created_at, bc.updated_at
		FROM workspace_pairings p
		JOIN stripe_connections sc ON sc.id = p.stripe_connection_id
		JOIN bokio_connections bc ON bc.id = p.bokio_connection_id
	` + suffix
}

func scanStripeOAuthState(row interface{ Scan(dest ...any) error }) (domain.StripeOAuthState, error) {
	var out domain.StripeOAuthState
	var usedAt sql.NullTime
	if err := row.Scan(&out.State, &out.WorkspaceID, &out.UserSubject, &out.ExpiresAt, &usedAt, &out.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return domain.StripeOAuthState{}, ErrNotFound
		}
		return domain.StripeOAuthState{}, fmt.Errorf("scan stripe oauth state: %w", err)
	}
	if usedAt.Valid {
		out.UsedAt = &usedAt.Time
	}
	return out, nil
}

func scanBokioOAuthState(row interface{ Scan(dest ...any) error }) (domain.BokioOAuthState, error) {
	var out domain.BokioOAuthState
	var requestedTenantID uuid.NullUUID
	var requestedTenantType sql.NullString
	var usedAt sql.NullTime
	if err := row.Scan(&out.State, &out.WorkspaceID, &out.UserSubject, &requestedTenantID, &requestedTenantType, &out.ExpiresAt, &usedAt, &out.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return domain.BokioOAuthState{}, ErrNotFound
		}
		return domain.BokioOAuthState{}, fmt.Errorf("scan bokio oauth state: %w", err)
	}
	if requestedTenantID.Valid {
		out.RequestedTenantID = &requestedTenantID.UUID
	}
	if requestedTenantType.Valid {
		out.RequestedTenantType = &requestedTenantType.String
	}
	if usedAt.Valid {
		out.UsedAt = &usedAt.Time
	}
	return out, nil
}

func scanStripeConnection(row interface{ Scan(dest ...any) error }) (domain.StripeConnection, error) {
	var out domain.StripeConnection
	var accountEmail, businessName, country sql.NullString
	var rawAccount []byte
	var disconnectedAt sql.NullTime
	if err := row.Scan(
		&out.ID, &out.WorkspaceID, &out.StripeAccountID, &out.StripeUserID, &out.Livemode, &out.Scope,
		&accountEmail, &businessName, &country, &rawAccount, &out.Status, &out.ConnectedAt, &disconnectedAt, &out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return domain.StripeConnection{}, ErrNotFound
		}
		return domain.StripeConnection{}, fmt.Errorf("scan stripe connection: %w", err)
	}
	if accountEmail.Valid {
		out.AccountEmail = &accountEmail.String
	}
	if businessName.Valid {
		out.BusinessName = &businessName.String
	}
	if country.Valid {
		out.Country = &country.String
	}
	if disconnectedAt.Valid {
		out.DisconnectedAt = &disconnectedAt.Time
	}
	out.RawAccount = rawAccount
	return out, nil
}

func scanBokioConnection(row interface{ Scan(dest ...any) error }) (domain.BokioConnection, error) {
	var out domain.BokioConnection
	var settings []byte
	var disconnectedAt sql.NullTime
	if err := row.Scan(
		&out.ID, &out.WorkspaceID, &out.BokioConnectionID, &out.BokioCompanyID, &out.CompanyName,
		&out.AccessTokenCipher, &out.RefreshTokenCipher, &out.TokenExpiresAt, &out.Scope, &settings,
		&out.SettingsVersion, &out.Status, &out.ConnectedAt, &disconnectedAt, &out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return domain.BokioConnection{}, ErrNotFound
		}
		return domain.BokioConnection{}, fmt.Errorf("scan bokio connection: %w", err)
	}
	if disconnectedAt.Valid {
		out.DisconnectedAt = &disconnectedAt.Time
	}
	out.Settings = settings
	return out, nil
}

func scanPairingRecord(row interface{ Scan(dest ...any) error }) (domain.PairingRecord, error) {
	var record domain.PairingRecord
	var pairingDisconnected sql.NullTime
	var stripeEmail, stripeBusinessName, stripeCountry sql.NullString
	var stripeRaw []byte
	var stripeDisconnected sql.NullTime
	var bokioSettings []byte
	var bokioDisconnected sql.NullTime
	if err := row.Scan(
		&record.Pairing.ID, &record.Pairing.WorkspaceID, &record.Pairing.StripeConnectionID, &record.Pairing.BokioConnectionID, &record.Pairing.Status, &record.Pairing.CreatedAt, &pairingDisconnected, &record.Pairing.UpdatedAt,
		&record.StripeConnection.ID, &record.StripeConnection.WorkspaceID, &record.StripeConnection.StripeAccountID, &record.StripeConnection.StripeUserID, &record.StripeConnection.Livemode, &record.StripeConnection.Scope, &stripeEmail, &stripeBusinessName, &stripeCountry, &stripeRaw, &record.StripeConnection.Status, &record.StripeConnection.ConnectedAt, &stripeDisconnected, &record.StripeConnection.CreatedAt, &record.StripeConnection.UpdatedAt,
		&record.BokioConnection.ID, &record.BokioConnection.WorkspaceID, &record.BokioConnection.BokioConnectionID, &record.BokioConnection.BokioCompanyID, &record.BokioConnection.CompanyName, &record.BokioConnection.AccessTokenCipher, &record.BokioConnection.RefreshTokenCipher, &record.BokioConnection.TokenExpiresAt, &record.BokioConnection.Scope, &bokioSettings, &record.BokioConnection.SettingsVersion, &record.BokioConnection.Status, &record.BokioConnection.ConnectedAt, &bokioDisconnected, &record.BokioConnection.CreatedAt, &record.BokioConnection.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return domain.PairingRecord{}, ErrNotFound
		}
		return domain.PairingRecord{}, fmt.Errorf("scan pairing record: %w", err)
	}
	if pairingDisconnected.Valid {
		record.Pairing.DisconnectedAt = &pairingDisconnected.Time
	}
	if stripeEmail.Valid {
		record.StripeConnection.AccountEmail = &stripeEmail.String
	}
	if stripeBusinessName.Valid {
		record.StripeConnection.BusinessName = &stripeBusinessName.String
	}
	if stripeCountry.Valid {
		record.StripeConnection.Country = &stripeCountry.String
	}
	record.StripeConnection.RawAccount = stripeRaw
	if stripeDisconnected.Valid {
		record.StripeConnection.DisconnectedAt = &stripeDisconnected.Time
	}
	record.BokioConnection.Settings = bokioSettings
	if bokioDisconnected.Valid {
		record.BokioConnection.DisconnectedAt = &bokioDisconnected.Time
	}
	return record, nil
}

func nullableTime(value interface{}) any {
	switch v := value.(type) {
	case nil:
		return nil
	case time.Time:
		if v.IsZero() {
			return nil
		}
		return v
	case sql.NullTime:
		if !v.Valid {
			return nil
		}
		return v.Time
	default:
		return value
	}
}
