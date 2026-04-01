CREATE INDEX IF NOT EXISTS idx_oss_union_entries_period
  ON oss_union_entries (bokio_company_id, filing_period, review_state);

CREATE INDEX IF NOT EXISTS idx_periodic_summary_entries_period
  ON periodic_summary_entries (bokio_company_id, filing_period, review_state);

CREATE INDEX IF NOT EXISTS idx_filing_periods_due
  ON filing_periods (bokio_company_id, first_send_at, submitted_at);

CREATE INDEX IF NOT EXISTS idx_filing_exports_latest
  ON filing_exports (bokio_company_id, kind, period, version DESC);
