CREATE TABLE IF NOT EXISTS tax_cases (
  id UUID PRIMARY KEY,
  bokio_company_id UUID NOT NULL,
  root_object_type TEXT NOT NULL,
  root_object_id TEXT NOT NULL,
  livemode BOOLEAN NOT NULL,
  source_currency CHAR(3),
  source_amount_minor BIGINT,
  sale_type TEXT,
  country CHAR(2),
  country_source TEXT,
  buyer_vat_number TEXT,
  buyer_vat_verified BOOLEAN NOT NULL DEFAULT FALSE,
  buyer_is_business BOOLEAN NOT NULL DEFAULT FALSE,
  tax_status TEXT,
  reportability_state TEXT NOT NULL CHECK (reportability_state IN ('reportable', 'needs_manual_evidence', 'needs_review')),
  review_reason TEXT,
  automatic_tax_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  automatic_tax_status TEXT,
  stripe_tax_amount_known BOOLEAN NOT NULL DEFAULT FALSE,
  stripe_tax_amount_minor BIGINT,
  stripe_tax_reverse_charge BOOLEAN NOT NULL DEFAULT FALSE,
  stripe_tax_zero_rated BOOLEAN NOT NULL DEFAULT FALSE,
  invoice_pdf_url TEXT,
  dossier JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (bokio_company_id, root_object_type, root_object_id)
);

CREATE TABLE IF NOT EXISTS tax_case_objects (
  tax_case_id UUID NOT NULL REFERENCES tax_cases(id) ON DELETE CASCADE,
  object_type TEXT NOT NULL,
  object_id TEXT NOT NULL,
  object_role TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tax_case_id, object_type, object_id)
);

CREATE TABLE IF NOT EXISTS manual_tax_evidence (
  id UUID PRIMARY KEY,
  tax_case_id UUID NOT NULL REFERENCES tax_cases(id) ON DELETE CASCADE,
  country CHAR(2),
  country_source TEXT,
  buyer_vat_number TEXT,
  buyer_vat_verified BOOLEAN,
  buyer_is_business BOOLEAN,
  sale_type TEXT,
  note TEXT,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE accounting_facts
  ADD COLUMN IF NOT EXISTS tax_case_id UUID REFERENCES tax_cases(id);

ALTER TABLE oss_union_entries
  ADD COLUMN IF NOT EXISTS tax_case_id UUID REFERENCES tax_cases(id);

ALTER TABLE periodic_summary_entries
  ADD COLUMN IF NOT EXISTS tax_case_id UUID REFERENCES tax_cases(id);

