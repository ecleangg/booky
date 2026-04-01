CREATE INDEX IF NOT EXISTS idx_stripe_events_unprocessed
  ON stripe_webhook_events (processed_at)
  WHERE processed_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_balance_transactions_available_on
  ON stripe_balance_transactions (available_on);

CREATE INDEX IF NOT EXISTS idx_balance_transactions_source_object
  ON stripe_balance_transactions (source_object_type, source_object_id);

CREATE INDEX IF NOT EXISTS idx_accounting_facts_pending_by_day
  ON accounting_facts (status, posting_date, bokio_company_id);

CREATE INDEX IF NOT EXISTS idx_accounting_facts_source_group
  ON accounting_facts (source_group_id);

CREATE INDEX IF NOT EXISTS idx_posting_runs_date
  ON posting_runs (bokio_company_id, posting_date);

CREATE UNIQUE INDEX IF NOT EXISTS uq_posting_runs_daily_close
  ON posting_runs (bokio_company_id, posting_date)
  WHERE run_type = 'daily_close';
