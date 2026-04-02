CREATE TABLE IF NOT EXISTS stripe_oauth_states (
  state TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL,
  user_subject TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS stripe_connections (
  id UUID PRIMARY KEY,
  workspace_id TEXT NOT NULL,
  stripe_account_id TEXT NOT NULL,
  stripe_user_id TEXT NOT NULL,
  livemode BOOLEAN NOT NULL,
  scope TEXT NOT NULL DEFAULT '',
  account_email TEXT,
  business_name TEXT,
  country TEXT,
  raw_account JSONB NOT NULL DEFAULT '{}'::jsonb,
  status TEXT NOT NULL CHECK (status IN ('active', 'disconnected')),
  connected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  disconnected_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_stripe_connections_active_account
  ON stripe_connections (stripe_account_id, livemode)
  WHERE status = 'active';

CREATE TABLE IF NOT EXISTS bokio_oauth_states (
  state TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL,
  user_subject TEXT NOT NULL,
  requested_tenant_id UUID,
  requested_tenant_type TEXT,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS bokio_connections (
  id UUID PRIMARY KEY,
  workspace_id TEXT NOT NULL,
  bokio_connection_id UUID NOT NULL,
  bokio_company_id UUID NOT NULL,
  company_name TEXT NOT NULL DEFAULT '',
  access_token_cipher TEXT NOT NULL,
  refresh_token_cipher TEXT NOT NULL,
  token_expires_at TIMESTAMPTZ NOT NULL,
  scope TEXT NOT NULL DEFAULT '',
  settings JSONB NOT NULL DEFAULT '{}'::jsonb,
  settings_version INTEGER NOT NULL DEFAULT 1,
  status TEXT NOT NULL CHECK (status IN ('active', 'disconnected')),
  connected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  disconnected_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bokio_connections_active_company
  ON bokio_connections (bokio_company_id)
  WHERE status = 'active';

CREATE UNIQUE INDEX IF NOT EXISTS idx_bokio_connections_active_connection
  ON bokio_connections (bokio_connection_id)
  WHERE status = 'active';

CREATE TABLE IF NOT EXISTS workspace_pairings (
  id UUID PRIMARY KEY,
  workspace_id TEXT NOT NULL,
  stripe_connection_id UUID NOT NULL REFERENCES stripe_connections(id),
  bokio_connection_id UUID NOT NULL REFERENCES bokio_connections(id),
  status TEXT NOT NULL CHECK (status IN ('active', 'disconnected')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  disconnected_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_pairings_active_stripe
  ON workspace_pairings (stripe_connection_id)
  WHERE status = 'active';

CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_pairings_active_bokio
  ON workspace_pairings (bokio_connection_id)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_workspace_pairings_workspace
  ON workspace_pairings (workspace_id, status);
