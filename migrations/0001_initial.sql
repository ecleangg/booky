CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS stripe_webhook_events (
  stripe_event_id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL,
  livemode BOOLEAN NOT NULL,
  api_version TEXT,
  stripe_created_at TIMESTAMPTZ NOT NULL,
  received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  payload JSONB NOT NULL,
  processed_at TIMESTAMPTZ,
  processing_error TEXT
);

CREATE TABLE IF NOT EXISTS stripe_object_snapshots (
  object_type TEXT NOT NULL,
  stripe_object_id TEXT NOT NULL,
  livemode BOOLEAN NOT NULL,
  payload JSONB NOT NULL,
  first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_synced_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (object_type, stripe_object_id)
);

CREATE TABLE IF NOT EXISTS stripe_balance_transactions (
  stripe_balance_transaction_id TEXT PRIMARY KEY,
  stripe_account_id TEXT NOT NULL,
  source_object_type TEXT,
  source_object_id TEXT,
  type TEXT NOT NULL,
  reporting_category TEXT,
  status TEXT,
  currency CHAR(3) NOT NULL,
  currency_exponent SMALLINT NOT NULL,
  amount_minor BIGINT NOT NULL,
  fee_minor BIGINT NOT NULL,
  net_minor BIGINT NOT NULL,
  exchange_rate NUMERIC(18,8),
  amount_sek_ore BIGINT,
  fee_sek_ore BIGINT,
  net_sek_ore BIGINT,
  occurred_at TIMESTAMPTZ NOT NULL,
  available_on DATE,
  payout_id TEXT,
  source_event_id TEXT,
  payload JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS accounting_facts (
  id UUID PRIMARY KEY,
  bokio_company_id UUID NOT NULL,
  stripe_account_id TEXT NOT NULL,
  source_group_id TEXT NOT NULL,
  source_object_type TEXT NOT NULL,
  source_object_id TEXT NOT NULL,
  stripe_balance_transaction_id TEXT,
  stripe_event_id TEXT,
  fact_type TEXT NOT NULL,
  posting_date DATE NOT NULL,
  market_code TEXT,
  vat_treatment TEXT,
  source_currency CHAR(3),
  source_amount_minor BIGINT,
  amount_sek_ore BIGINT NOT NULL,
  bokio_account INTEGER NOT NULL,
  direction TEXT NOT NULL CHECK (direction IN ('debit', 'credit')),
  status TEXT NOT NULL CHECK (status IN ('pending', 'batched', 'posted', 'reversed', 'failed', 'needs_review')),
  review_reason TEXT,
  payload JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (source_group_id, fact_type, bokio_account, direction, posting_date)
);

CREATE TABLE IF NOT EXISTS posting_runs (
  id UUID PRIMARY KEY,
  bokio_company_id UUID NOT NULL,
  posting_date DATE NOT NULL,
  timezone TEXT NOT NULL,
  run_type TEXT NOT NULL CHECK (run_type IN ('daily_close', 'repost')),
  sequence_no INTEGER NOT NULL DEFAULT 1,
  status TEXT NOT NULL CHECK (status IN ('started', 'journal_created', 'upload_created', 'completed', 'failed')),
  config_snapshot JSONB NOT NULL,
  summary JSONB NOT NULL DEFAULT '{}'::jsonb,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  finished_at TIMESTAMPTZ,
  error_message TEXT,
  UNIQUE (bokio_company_id, posting_date, run_type, sequence_no)
);

CREATE TABLE IF NOT EXISTS posting_run_facts (
  posting_run_id UUID NOT NULL REFERENCES posting_runs(id),
  accounting_fact_id UUID NOT NULL REFERENCES accounting_facts(id),
  PRIMARY KEY (posting_run_id, accounting_fact_id)
);

CREATE TABLE IF NOT EXISTS bokio_journals (
  posting_run_id UUID PRIMARY KEY REFERENCES posting_runs(id),
  bokio_company_id UUID NOT NULL,
  bokio_journal_entry_id UUID NOT NULL,
  bokio_journal_entry_number TEXT,
  bokio_upload_id UUID,
  bokio_journal_title TEXT NOT NULL,
  posting_date DATE NOT NULL,
  attachment_checksum TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  reversed_at TIMESTAMPTZ,
  reversed_by_journal_entry_id UUID
);
