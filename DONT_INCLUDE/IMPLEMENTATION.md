# Implementation Breakdown

This document turns the agreed accounting plan into a concrete starting point for development.

Confirmed decisions from the earlier review:

1. Book the full voucher in Bokio from Stripe data.
2. Move `1580 -> Stripe balance account` on Stripe `available_on`.
3. Create one Bokio voucher per Stockholm day, summarized in SEK.
4. Post unknown Stripe categories to the configured OBS account.
5. Book refunds and disputes automatically.
6. Use one Stripe balance account per currency.
7. Use Stripe-settled SEK values as the journal basis.
8. Keep payouts in the same day-level voucher when they occur that day.
9. Derive market and VAT treatment from Stripe tax and customer data.

## Repository Layout

Proposed Go module layout:

```text
booky/
  cmd/
    booky/
      main.go
  internal/
    app/
      app.go
    config/
      config.go
      validate.go
    clock/
      clock.go
    httpapi/
      router.go
      health.go
      stripe_webhook.go
      admin_runs.go
    jobs/
      daily_close.go
      backfill.go
    store/
      db.go
      tx.go
      advisory_lock.go
      stripe_events.go
      stripe_objects.go
      stripe_balance_transactions.go
      accounting_facts.go
      posting_runs.go
      bokio_journals.go
    stripe/
      client.go
      webhook.go
      balance_transactions.go
      expand.go
    classify/
      market.go
      vat.go
      source_data.go
    accounting/
      normalize.go
      buckets.go
      accounts.go
      batch.go
      journal.go
      amounts.go
      rounding.go
    bokio/
      client.go
      journal_entries.go
      uploads.go
      chart_of_accounts.go
      fiscal_years.go
    pdf/
      journal_attachment.go
    domain/
      stripe.go
      accounting.go
      bokio.go
    observability/
      logger.go
      metrics.go
  migrations/
    0001_initial.sql
    0002_indexes.sql
  testdata/
    stripe/
      sample_day.json
      disputes.json
      refunds.json
    bokio/
      journal_created.json
  SETTINGS.md
  VERIFIKAT_EXEMPEL.md
  bokio-company-api.yaml
  IMPLEMENTATION.md
```

## Package Responsibilities

### `cmd/booky`

Program entrypoint. Wire config, database, HTTP server, Stripe client, Bokio client, and scheduler.

### `internal/app`

Application bootstrap and dependency construction. Keep `main.go` thin.

### `internal/config`

Load YAML plus environment overrides. Validate that all required account mappings exist before startup.

Important validations:

1. Every configured market has a revenue account.
2. Markets with VAT require an output VAT account.
3. `EU_B2B` and `EXPORT` have revenue accounts and no output VAT account.
4. Every active Stripe balance currency has a Stripe balance account.
5. `1580`, bank, fee, reverse-charge VAT, dispute, OBS, and rounding accounts are present.

### `internal/httpapi`

Minimal HTTP layer.

Endpoints for phase 1:

1. `GET /healthz`
2. `POST /webhooks/stripe`
3. `POST /admin/runs/daily-close?date=YYYY-MM-DD` protected by admin bearer auth

Use `net/http` directly unless a router is needed for readability.

### `internal/jobs`

Background execution:

1. Daily close scheduler.
2. Optional backfill worker for replaying old Stripe data.

### `internal/store`

PostgreSQL access via `pgx/v5`.

Rules:

1. All writes happen in explicit transactions.
2. Use Postgres advisory locks for the daily-close job.
3. Keep SQL close to the repo layer. Do not hide every query behind heavy abstractions.

### `internal/stripe`

Stripe integration:

1. Verify webhook signatures.
2. Fetch and expand Stripe objects required for accounting.
3. Normalize balance transaction data into stable internal records.

Accounting truth should come from Stripe balance transactions plus expanded source objects.

### `internal/classify`

Resolve accounting classification from Stripe data.

Outputs:

1. Market code: `SE`, `BE`, `EU_B2B`, `EXPORT`, etc.
2. VAT treatment.
3. Whether the transaction is safe to auto-book.
4. Review reason when it is not safe.

### `internal/accounting`

Core ledger logic. This package converts Stripe records into pending accounting facts, then turns same-day facts into Bokio journal items.

Important split:

1. `normalize.go` creates pending accounting facts from Stripe events and Stripe objects.
2. `batch.go` selects pending facts for one Stockholm day.
3. `journal.go` groups facts by Bokio account into debit and credit totals.
4. `rounding.go` adds a final balancing row when ore differences remain.

### `internal/bokio`

Thin Bokio API client:

1. Check chart of accounts.
2. Check open fiscal year.
3. Create journal entry.
4. Upload PDF attachment.
5. Reverse API-created journal entries if needed.

### `internal/pdf`

Build one PDF per Bokio journal.

The PDF must include:

1. Posting date and Bokio company.
2. Included Stripe transaction IDs and event IDs.
3. Source currency amount, fee, net, and settled SEK amount.
4. Summary by accounting bucket.
5. A stable checksum of included facts.

### `internal/domain`

Shared structs used across packages. Keep them small and explicit.

### `internal/observability`

Structured logging and metrics.

## Configuration Shape

Recommended config file: `config/booky.yaml`

Example shape:

```yaml
app:
  env: dev
  timezone: Europe/Stockholm

http:
  listen_addr: ":8080"

postgres:
  dsn: postgresql://booky:booky@localhost:5432/booky?sslmode=disable

stripe:
  api_key: ${STRIPE_API_KEY}
  webhook_secret: ${STRIPE_WEBHOOK_SECRET}
  account_id: acct_xxx

bokio:
  company_id: 00000000-0000-0000-0000-000000000000
  token: ${BOKIO_TOKEN}

posting:
  cutoff_time: "23:59"
  auto_post_unknown_to_obs: false
  fx_source: stripe_settled_sek

accounts:
  stripe_receivable: 1580
  bank: 1920
  dispute: 1510
  fallback_obs: 2999
  rounding: 3740

  stripe_balance_by_currency:
    SEK: 1980
    EUR: 1981
    USD: 1982

  stripe_fees:
    expense: 4535
    input_vat: 2645
    output_vat: 2614

  sales_by_market:
    SE:
      revenue: 3001
      output_vat: 2611
    BE:
      revenue: 3105
      output_vat: 2614
    EU_B2B:
      revenue: 3308
    EXPORT:
      revenue: 3305
```

Notes:

1. Keep actual account numbers in config, not code.
2. Market keys should match what `internal/classify` returns.
3. Add env overrides for all secrets and DSNs.

## Data Model

The cleanest implementation is two-stage persistence:

1. Persist raw Stripe evidence.
2. Persist normalized accounting facts, one row per future Bokio journal contribution.

This keeps posting idempotent and makes the PDF easy to build from already-classified rows.

### Table: `stripe_webhook_events`

Stores deduplicated raw Stripe events.

```sql
CREATE TABLE stripe_webhook_events (
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
```

### Table: `stripe_object_snapshots`

Stores the latest fetched Stripe object required for classification.

```sql
CREATE TABLE stripe_object_snapshots (
  object_type TEXT NOT NULL,
  stripe_object_id TEXT NOT NULL,
  livemode BOOLEAN NOT NULL,
  payload JSONB NOT NULL,
  first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_synced_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (object_type, stripe_object_id)
);
```

Examples of `object_type`:

1. `charge`
2. `refund`
3. `dispute`
4. `payout`
5. `invoice`
6. `customer`
7. `balance_transaction`

### Table: `stripe_balance_transactions`

One row per Stripe balance transaction used as accounting evidence.

Store all source-currency values in minor units and all journal values in SEK ore.

```sql
CREATE TABLE stripe_balance_transactions (
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
```

Notes:

1. `amount_sek_ore`, `fee_sek_ore`, and `net_sek_ore` should be filled from Stripe-settled SEK values where available.
2. If a non-SEK transaction cannot be resolved to a safe SEK amount, mark downstream facts for review.

### Table: `accounting_facts`

This is the main pending queue. Each row is one future Bokio debit or credit contribution.

Example:

1. One charge can produce three facts for sale recognition: `1580 debit`, `sales credit`, `VAT credit`.
2. The same charge can later produce two facts on `available_on`: `1980 debit`, `1580 credit`.
3. A Stripe fee can produce four facts: expense debit, Stripe balance credit, input VAT debit, output VAT credit.

```sql
CREATE TABLE accounting_facts (
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
```

Recommended `fact_type` values:

`posting_date` is the accounting date in `Europe/Stockholm`.

1. `sale_receivable`
2. `sale_revenue`
3. `sale_output_vat`
4. `refund_receivable`
5. `refund_revenue`
6. `refund_output_vat`
7. `stripe_available_debit`
8. `stripe_available_credit`
9. `stripe_fee_expense`
10. `stripe_fee_balance`
11. `stripe_fee_input_vat`
12. `stripe_fee_output_vat`
13. `payout_bank`
14. `payout_stripe_balance`
15. `dispute_debit`
16. `dispute_credit`
17. `unknown_obs`
18. `rounding`

### Table: `posting_runs`

One row per daily posting attempt.

`daily_close` should use `sequence_no = 1`. Reposts for the same day should increment `sequence_no`.

```sql
CREATE TABLE posting_runs (
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
```

### Table: `posting_run_facts`

Links facts to the run that posted them.

```sql
CREATE TABLE posting_run_facts (
  posting_run_id UUID NOT NULL REFERENCES posting_runs(id),
  accounting_fact_id UUID NOT NULL REFERENCES accounting_facts(id),
  PRIMARY KEY (posting_run_id, accounting_fact_id)
);
```

### Table: `bokio_journals`

Stores Bokio IDs and upload IDs so retries can resume safely.

```sql
CREATE TABLE bokio_journals (
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
```

## Indexes

Create these in `0002_indexes.sql`:

```sql
CREATE INDEX idx_stripe_events_unprocessed
  ON stripe_webhook_events (processed_at)
  WHERE processed_at IS NULL;

CREATE INDEX idx_balance_transactions_available_on
  ON stripe_balance_transactions (available_on);

CREATE INDEX idx_balance_transactions_source_object
  ON stripe_balance_transactions (source_object_type, source_object_id);

CREATE INDEX idx_accounting_facts_pending_by_day
  ON accounting_facts (status, posting_date, bokio_company_id);

CREATE INDEX idx_accounting_facts_source_group
  ON accounting_facts (source_group_id);

CREATE INDEX idx_posting_runs_date
  ON posting_runs (bokio_company_id, posting_date);

CREATE UNIQUE INDEX uq_posting_runs_daily_close
  ON posting_runs (bokio_company_id, posting_date)
  WHERE run_type = 'daily_close';
```

## Domain Flow

The important design choice is that one Stripe business event can create multiple accounting facts with different posting dates.

### Example: successful charge

On charge date:

1. `1580 debit` for gross sale amount.
2. Revenue account credit for net sales amount excluding VAT.
3. Output VAT account credit for VAT amount.
4. Stripe fee expense debit.
5. Stripe balance account credit for fee amount.
6. Reverse-charge VAT on Stripe fee.

On Stripe `available_on` date:

1. Stripe balance account debit for gross funds becoming available.
2. `1580` credit for same amount.

On payout date:

1. Bank account debit.
2. Stripe balance account credit.

### Example: refund

On refund date:

1. Reverse sale receivable.
2. Reverse revenue.
3. Reverse output VAT.
4. If Stripe fee behavior differs, book fee impact separately from the balance transaction evidence.

### Example: dispute

On dispute open date:

1. Debit dispute account.
2. Credit Stripe balance or receivable depending on the Stripe movement.

If the dispute later closes differently, create correction facts and let the daily-close job post them normally.

## Posting Engine

Daily-close algorithm for one Bokio company and one Stockholm day:

1. Acquire Postgres advisory lock for `company + date + daily_close`.
2. Read config and snapshot it into a `posting_runs` row.
3. Validate Bokio chart of accounts for all accounts used by pending facts.
4. Validate the fiscal year for `posting_date` is open.
5. Select all `accounting_facts.status = 'pending'` for the day.
6. Split any `needs_review` facts:
   1. If config says auto-post to OBS, they should already carry the OBS account.
   2. Otherwise fail before journal creation.
7. Group facts by `bokio_account` and `direction`, sum `amount_sek_ore`.
8. If total debit and credit differ by <= configured tolerance, add one rounding fact.
9. Create Bokio journal entry.
10. Generate PDF from the exact included facts.
11. Upload PDF to Bokio with `journalEntryId`.
12. Mark facts as `posted`, create `bokio_journals`, and complete the run.

Failure handling:

1. If Bokio journal creation fails, keep facts `pending` and mark the run `failed`.
2. If journal creation succeeds but PDF upload fails, keep the Bokio journal ID in `bokio_journals` and retry upload only.
3. Never create a second Bokio journal for the same `posting_run` once one exists.

## Bokio Client Contract

The Bokio API requires only a small surface for phase 1.

### Journal create request

Use:

`POST /companies/{companyId}/journal-entries`

Payload shape:

```json
{
  "title": "Stripe dagsverifikat 2026-04-01",
  "date": "2026-04-01",
  "items": [
    {"account": 1580, "debit": 12500.00, "credit": 0},
    {"account": 3001, "debit": 0, "credit": 10000.00},
    {"account": 2611, "debit": 0, "credit": 2500.00}
  ]
}
```

Amounts are in SEK. Internally keep ore until the HTTP boundary, then render `SEK ore / 100`.

### Upload request

Use:

`POST /companies/{companyId}/uploads`

Multipart form fields:

1. `file`
2. `journalEntryId`
3. `description`

Description example:

`Stripe dagsverifikat 2026-04-01 underlag`

## PDF Attachment Layout

Keep the first version plain and audit-friendly, not pretty.

Recommended sections:

1. Header:
   1. Bokio company ID
   2. Posting date
   3. Generated at
   4. Run ID
2. Bokio journal summary:
   1. Account
   2. Debit SEK
   3. Credit SEK
3. Included Stripe facts:
   1. Stripe event ID
   2. Balance transaction ID
   3. Source object ID
   4. Fact type
   5. Market code
   6. Source currency amount
   7. SEK amount
4. Included raw Stripe transactions:
   1. Type
   2. Reporting category
   3. Gross
   4. Fee
   5. Net
   6. Available date
5. Final checksum

## Suggested Dependencies

Keep dependencies small.

Recommended:

1. `github.com/jackc/pgx/v5` for Postgres.
2. `github.com/stripe/stripe-go` for Stripe.
3. `gopkg.in/yaml.v3` for config parsing.
4. A simple PDF library such as `github.com/go-pdf/fpdf`.

Use the standard library for:

1. HTTP server
2. Context handling
3. Logging if no stronger requirement exists
4. Multipart file upload to Bokio

## Full Development Steps

### Phase 0: bootstrap

1. Initialize Go module.
2. Add `.gitignore` for build outputs and local config.
3. Add `config/booky.example.yaml` from the config shape above.
4. Add `migrations/0001_initial.sql` and `0002_indexes.sql`.
5. Add `docker-compose.yml` for local Postgres.
6. Add `cmd/booky/main.go` with startup, config loading, DB ping, and `GET /healthz`.

Deliverable:

Service starts locally, loads config, connects to Postgres, and runs migrations.

### Phase 1: Stripe webhook intake

1. Implement `POST /webhooks/stripe`.
2. Verify Stripe signature.
3. Upsert into `stripe_webhook_events` by `stripe_event_id`.
4. Return `200` for duplicate events after dedupe.
5. Mark events as `processed_at` only after downstream normalization succeeds.

Deliverable:

Webhook events are durably stored and idempotent.

### Phase 2: Stripe object syncing

1. Add Stripe client wrapper.
2. For supported incoming event types, fetch current Stripe source objects and balance transactions.
3. Upsert raw object payloads into `stripe_object_snapshots`.
4. Upsert resolved balance transaction data into `stripe_balance_transactions`.

Start with these event types:

1. `charge.succeeded`
2. `charge.updated`
3. `charge.refunded`
4. `charge.dispute.created`
5. `charge.dispute.closed`
6. `payout.paid`
7. `payout.failed`
8. `balance.available`

Deliverable:

The database contains the Stripe evidence needed to derive accounting facts.

### Phase 3: classification and normalization

1. Build `internal/classify` to derive market and VAT treatment from Stripe tax and customer fields.
2. Build `internal/accounting/normalize.go` to create `accounting_facts`.
3. Use stable `source_group_id` values such as:
   1. `charge:ch_xxx:sale`
   2. `charge:ch_xxx:available`
   3. `charge:ch_xxx:fee`
   4. `payout:po_xxx`
   5. `refund:re_xxx`
   6. `dispute:dp_xxx`
4. Ensure normalization is idempotent using the `UNIQUE` constraint on `accounting_facts`.
5. Route unsupported cases to OBS facts with `review_reason` populated.

Deliverable:

For each supported Stripe flow, pending accounting facts exist and can be inspected before posting.

### Phase 4: daily-close runner

1. Implement `internal/jobs/daily_close.go`.
2. Compute Stockholm day boundaries.
3. Acquire Postgres advisory lock.
4. Create `posting_runs` row.
5. Select and batch pending facts.
6. Summarize grouped Bokio journal items.
7. Add rounding line when needed.

Deliverable:

The service can produce the exact Bokio request body for one day without calling Bokio yet.

### Phase 5: Bokio integration

1. Implement chart-of-accounts validation.
2. Implement fiscal-year validation.
3. Implement journal creation.
4. Persist `bokio_journals` row after successful journal creation.
5. Implement retry-safe behavior so a second run does not create a duplicate journal.

Deliverable:

One day of pending facts creates one Bokio journal entry.

### Phase 6: PDF attachment

1. Implement `internal/pdf/journal_attachment.go`.
2. Render the PDF from the exact set of posted facts.
3. Upload via Bokio uploads endpoint with `journalEntryId`.
4. Save upload ID and checksum.

Deliverable:

Each Bokio journal has one attached PDF listing all covered Stripe evidence.

### Phase 7: reversal and correction workflow

1. Implement reversal command using Bokio reverse endpoint.
2. Mark local facts and `bokio_journals` as reversed.
3. Support generating a corrected repost for the same day.

Deliverable:

The service can reverse API-created journals and post corrected replacements.

### Phase 8: operational hardening

1. Metrics for webhook volume, pending facts, posting failures, and Bokio API latency.
2. Structured logs with Stripe IDs, Bokio IDs, run IDs, and posting dates.
3. Admin endpoint or CLI for replaying one event or one posting day.
4. Dead-letter or review tooling for repeated classification failures.

Deliverable:

The service is supportable in production.

## First Development Slice

The smallest useful vertical slice is:

1. Bootstrap app and Postgres.
2. Store Stripe webhook events.
3. Sync charges, payouts, and balance transactions.
4. Normalize one successful charge flow into accounting facts.
5. Build one daily voucher request body in memory.

Do not start with disputes or reversals first. Build the normal charge -> fee -> available -> payout path end to end before expanding edge cases.

## Initial Milestone Scope

Milestone 1 should support:

1. `charge.succeeded`
2. Stripe fees
3. `available_on` receivable release
4. `payout.paid`
5. Bokio journal creation
6. Bokio PDF upload

Milestone 2 should add:

1. Refunds
2. Disputes
3. Reversal and repost
4. Better admin and replay tooling

## Testing Strategy

### Unit tests

1. Market classification from Stripe tax/customer inputs.
2. VAT mapping for `SE`, `EU_B2B`, `EXPORT`.
3. Charge normalization into expected accounting facts.
4. Rounding behavior.
5. Journal grouping output.

### Repository tests

1. Idempotent webhook insert.
2. Idempotent normalization.
3. Pending fact selection by posting day.
4. Posting run locking.

### Integration tests

1. End-to-end run from Stripe fixture to Bokio request payload.
2. Partial failure after journal create and before upload.
3. Re-run of the same day does not create duplicate Bokio journal.

### Golden tests

Use `VERIFIKAT_EXEMPEL.md` as a reference fixture for expected grouped accounting output.

## Definition of Done for First Production Version

1. A Stripe webhook can be replayed safely any number of times.
2. One Stockholm day creates at most one Bokio daily-close run.
3. One posting run creates at most one Bokio journal.
4. All included Stripe source data can be traced from the PDF and database.
5. Unmappable cases are never silently dropped.
6. Bokio fiscal-year and account validation happen before journal creation.
7. All values posted to Bokio are balanced in SEK.

## Recommended Next File Additions

After this document, the next concrete files to add are:

1. `go.mod`
2. `cmd/booky/main.go`
3. `internal/config/config.go`
4. `migrations/0001_initial.sql`
5. `config/booky.example.yaml`

That is the minimum scaffold needed to start implementation cleanly.
