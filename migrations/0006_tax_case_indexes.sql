CREATE INDEX IF NOT EXISTS idx_tax_cases_company_root
  ON tax_cases (bokio_company_id, root_object_type, root_object_id);

CREATE INDEX IF NOT EXISTS idx_tax_cases_status
  ON tax_cases (bokio_company_id, tax_status, reportability_state);

CREATE INDEX IF NOT EXISTS idx_tax_case_objects_lookup
  ON tax_case_objects (object_type, object_id);

CREATE INDEX IF NOT EXISTS idx_manual_tax_evidence_case
  ON manual_tax_evidence (tax_case_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_accounting_facts_tax_case
  ON accounting_facts (tax_case_id);

CREATE INDEX IF NOT EXISTS idx_oss_union_entries_tax_case
  ON oss_union_entries (tax_case_id);

CREATE INDEX IF NOT EXISTS idx_periodic_summary_entries_tax_case
  ON periodic_summary_entries (tax_case_id);
