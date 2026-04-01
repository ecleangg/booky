CREATE TABLE IF NOT EXISTS oss_union_entries (
  id UUID PRIMARY KEY,
  bokio_company_id UUID NOT NULL,
  source_group_id TEXT NOT NULL UNIQUE,
  source_object_type TEXT NOT NULL,
  source_object_id TEXT NOT NULL,
  stripe_event_id TEXT,
  original_supply_period TEXT NOT NULL,
  filing_period TEXT NOT NULL,
  correction_target_period TEXT,
  consumption_country CHAR(2) NOT NULL,
  origin_country CHAR(2) NOT NULL,
  origin_identifier TEXT NOT NULL,
  sale_type TEXT NOT NULL CHECK (sale_type IN ('GOODS', 'SERVICES')),
  vat_rate_basis_points INTEGER NOT NULL,
  taxable_amount_eur_cents BIGINT NOT NULL,
  vat_amount_eur_cents BIGINT NOT NULL,
  review_state TEXT NOT NULL CHECK (review_state IN ('ready', 'review')),
  review_reason TEXT,
  payload JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS periodic_summary_entries (
  id UUID PRIMARY KEY,
  bokio_company_id UUID NOT NULL,
  source_group_id TEXT NOT NULL UNIQUE,
  source_object_type TEXT NOT NULL,
  source_object_id TEXT NOT NULL,
  stripe_event_id TEXT,
  filing_period TEXT NOT NULL,
  buyer_vat_number TEXT NOT NULL,
  row_type TEXT NOT NULL CHECK (row_type IN ('goods', 'services')),
  amount_sek_ore BIGINT NOT NULL,
  exported_amount_sek BIGINT NOT NULL,
  review_state TEXT NOT NULL CHECK (review_state IN ('ready', 'review')),
  review_reason TEXT,
  payload JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS filing_periods (
  kind TEXT NOT NULL,
  period TEXT NOT NULL,
  bokio_company_id UUID NOT NULL,
  deadline_date DATE NOT NULL,
  first_send_at TIMESTAMPTZ NOT NULL,
  last_evaluated_at TIMESTAMPTZ,
  last_evaluation_status TEXT NOT NULL DEFAULT 'pending',
  zero_reminder_sent_at TIMESTAMPTZ,
  submitted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (kind, period, bokio_company_id)
);

CREATE TABLE IF NOT EXISTS filing_exports (
  id UUID PRIMARY KEY,
  kind TEXT NOT NULL,
  period TEXT NOT NULL,
  bokio_company_id UUID NOT NULL,
  version INTEGER NOT NULL,
  checksum TEXT NOT NULL,
  filename TEXT,
  content BYTEA,
  summary JSONB NOT NULL DEFAULT '{}'::jsonb,
  emailed_at TIMESTAMPTZ,
  superseded_by UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (kind, period, bokio_company_id, version)
);
