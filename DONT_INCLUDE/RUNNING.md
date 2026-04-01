# Running Guide

This guide explains how to run `booky` in:

1. local development mode
2. production mode
3. Stripe test mode against a Bokio test company

It assumes the current implementation in this repository:

1. receives Stripe webhooks
2. stores raw events and normalized accounting facts in PostgreSQL
3. creates Bokio journal entries through the Bokio Company API
4. uploads one PDF attachment per journal

## Prerequisites

Install:

1. Go `1.26+`
2. Docker Desktop or another Docker runtime
3. PostgreSQL client tools if you want to inspect the database manually
4. Stripe CLI

You also need:

1. one Stripe test account with API key and webhook secret
2. one Bokio test company and API token with these scopes:
   1. `journal-entries:write`
   2. `uploads:write`
   3. `chart-of-accounts:read`
   4. `fiscal-years:read`

## Files You Need

Relevant files in this repo:

1. `config/booky.example.yaml`
2. `SETTINGS.md`
3. `IMPLEMENTATION.md`
4. `RUNNING.md`

Create your local config from the example:

```powershell
Copy-Item "config/booky.example.yaml" "config/booky.yaml"
```

## Automated Tests

The repository now includes unit tests, transport tests, workflow tests, and PostgreSQL-backed integration tests.

Run the fast local suite with:

```powershell
go test ./...
```

Repository and workflow integration tests use `BOOKY_TEST_POSTGRES_DSN`.

If the variable is not set, those tests are skipped locally by design.

To run the same validation stack as CI:

```powershell
docker compose up -d postgres
$env:BOOKY_TEST_POSTGRES_DSN = "postgresql://booky:booky@localhost:5432/booky?sslmode=disable"
go build ./cmd/booky
go vet ./...
go test ./...
```

CI runs the same commands on every `push` and `pull_request` with PostgreSQL `17`, using the same `BOOKY_TEST_POSTGRES_DSN` convention.

## Dev Mode

### 1. Start PostgreSQL

From the repo root:

```powershell
docker compose up -d postgres
```

Verify it is running:

```powershell
docker compose ps
```

### 2. Set secrets

PowerShell example:

```powershell
$env:BOOKY_POSTGRES_DSN = "postgresql://booky:booky@localhost:5432/booky?sslmode=disable"
$env:BOOKY_ADMIN_TOKEN = "change-me"
$env:STRIPE_API_KEY = "sk_test_..."
$env:STRIPE_WEBHOOK_SECRET = "whsec_..."
$env:STRIPE_ACCOUNT_ID = "acct_..."
$env:BOKIO_COMPANY_ID = "00000000-0000-0000-0000-000000000000"
$env:BOKIO_TOKEN = "..."
```

If you use a Bokio test company, set its UUID in `config/booky.yaml`.

### 3. Configure accounts

Edit `config/booky.yaml` and fill in:

1. `bokio.company_id`
2. `postgres.dsn`
3. `accounts.sales_by_market`
4. `accounts.stripe_balance_by_currency`
5. fee, VAT, bank, dispute, OBS, and rounding accounts

These must match the Bokio test company chart of accounts.

### 4. Start the API

```powershell
go run ./cmd/booky
```

By default it reads:

`config/booky.yaml`

To use another config file:

```powershell
$env:BOOKY_CONFIG = "config/booky.dev.yaml"
go run ./cmd/booky
```

### 5. Verify startup

Health check:

```powershell
Invoke-RestMethod "http://localhost:8080/healthz"
```

Expected response:

```json
{"status":"ok"}
```

### 6. What happens on startup

When the service starts it:

1. loads config
2. connects to PostgreSQL
3. runs SQL migrations from `migrations/`
4. starts the HTTP server
5. starts the scheduler if `posting.scheduler_enabled: true`

For local testing, it is usually safer to leave the scheduler disabled and trigger daily close manually.

## Local Dev Config Recommendation

Use this pattern for `config/booky.yaml` in development:

```yaml
app:
  env: dev
  timezone: Europe/Stockholm

http:
  listen_addr: ":8080"
  read_timeout: 15s
  write_timeout: 30s
  idle_timeout: 60s

admin:
  enabled: true
  bearer_token: ${BOOKY_ADMIN_TOKEN}

postgres:
  dsn: ${BOOKY_POSTGRES_DSN}

stripe:
  api_key: ${STRIPE_API_KEY}
  webhook_secret: ${STRIPE_WEBHOOK_SECRET}
  account_id: ""
  api_base_url: https://api.stripe.com

bokio:
  company_id: 00000000-0000-0000-0000-000000000000
  token: ${BOKIO_TOKEN}
  base_url: https://api.bokio.se/v1

posting:
  cutoff_time: "23:59"
  scheduler_enabled: false
  scheduler_poll_interval: 5m
  auto_post_unknown_to_obs: false
  rounding_tolerance_ore: 2

notifications:
  resend:
    enabled: false
    api_key: ${RESEND_API_KEY}
    from: bookkeeping@example.com
    to:
      - finance@example.com
    base_url: https://api.resend.com
    subject_prefix: "[booky]"

accounts:
  other_countries_default:
    revenue: 3100
    output_vat: 2614
    vat_rate_percent: 25
```

`accounts.other_countries_default` is optional. Use it only when your upstream Stripe metadata explicitly marks the sale as safe for that fallback. Unsupported cross-border VAT cases now route to review instead of being guessed automatically.

`notifications.resend` is optional. If enabled, `booky` sends warning/error emails for daily-close issues such as OBS postings, rounding lines, and failed runs.

## How Review Cases Work Today

`booky` currently uses `needs_review` as a holding state when it is not safe to auto-book from Stripe evidence.

Important:

1. there is currently no separate UI to approve an `in_review` or `needs_review` item
2. the normal workflow is not to edit PostgreSQL rows by hand
3. the normal workflow is:
   1. inspect the `review_reason`
   2. correct the source of truth or config
   3. trigger fresh normalization
   4. rerun daily close

In practice that means:

1. If the review is caused by missing Stripe tax, country, customer, or VAT-ID evidence:
   1. correct the upstream Stripe/customer metadata or the integration that writes it
   2. trigger a fresh Stripe event for the object, for example by updating the charge metadata so Stripe emits `charge.updated`
2. If the review is caused by missing Bokio account mappings:
   1. update `config/booky.yaml` or the deployed config
   2. restart the service if needed so the new config is loaded
   3. trigger fresh normalization and rerun the day
3. If the transaction was already booked manually and should stay manual:
   1. make the correction in Bokio
   2. update the Stripe/source side so `booky` will not continue producing the same incorrect auto-booking facts

Rerun the blocked day with:

```powershell
Invoke-RestMethod -Method Post `
  -Headers @{ Authorization = "Bearer $env:BOOKY_ADMIN_TOKEN" } `
  "http://localhost:8080/admin/runs/daily-close?date=2026-04-01"
```

Use the Stockholm accounting date from the notification.

## Testing With Stripe Test Mode

The safest end-to-end test setup is:

1. Stripe test mode
2. local `booky`
3. Bokio test company

### 1. Forward Stripe webhooks locally

Use Stripe CLI:

```powershell
stripe listen --forward-to localhost:8080/webhooks/stripe
```

Stripe CLI prints a webhook signing secret similar to:

`whsec_...`

Use that value as `STRIPE_WEBHOOK_SECRET` for your local run.

Important:

1. the CLI signing secret is different from a dashboard-managed endpoint secret
2. if you restart `stripe listen`, the secret may change

### 2. Trigger test events

Start with a successful charge flow:

```powershell
stripe trigger charge.succeeded
```

Also test payout-related events when needed:

```powershell
stripe trigger payout.paid
```

Also test refund flow:

```powershell
stripe trigger charge.refunded
```

Also test dispute creation:

```powershell
stripe trigger charge.dispute.created
```

Important:

1. `stripe trigger ...` uses canned fixtures and commonly defaults to `USD`
2. for accounting tests in `SEK` and `EUR`, create real test-mode PaymentIntents instead of relying on canned triggers

### 2a. Create SEK and EUR test payments

Use Stripe CLI to create real successful payments in a chosen currency.

SEK example:

```powershell
stripe payment_intents create --amount 12500 --currency sek --payment-method pm_card_visa --confirm
```

EUR example:

```powershell
stripe payment_intents create --amount 12500 --currency eur --payment-method pm_card_visa --confirm
```

Notes:

1. `12500` means `125.00` in the payment currency minor units
2. these calls emit the normal Stripe events such as `payment_intent.succeeded` and `charge.succeeded`
3. they are better than `stripe trigger charge.succeeded` when you need specific currency behavior

### 2b. Create refunds for SEK and EUR test payments

After creating a payment, get the related charge ID:

```powershell
stripe payment_intents retrieve pi_xxx
```

Then refund the charge:

```powershell
stripe refunds create --charge ch_xxx
```

### 2c. Payout and dispute notes

1. `stripe trigger payout.paid` is still useful for payout-shape testing, but it is fixture-based
2. disputes are also easiest through Stripe trigger fixtures rather than custom currency creation
3. for sales-accounting currency tests, use `payment_intents create --currency sek|eur`

### 3. Verify webhook intake

Watch service logs for:

1. accepted Stripe event
2. fetched charge, payout, or balance transaction
3. stored accounting facts

Then check the database if needed.

Example using `psql`:

```sql
SELECT stripe_event_id, event_type, processed_at, processing_error
FROM stripe_webhook_events
ORDER BY received_at DESC;

SELECT source_group_id, fact_type, posting_date, bokio_account, direction, amount_sek_ore, status
FROM accounting_facts
ORDER BY created_at DESC;
```

### 4. Trigger daily close manually

If you want to post today’s facts to Bokio manually:

```powershell
Invoke-RestMethod -Method Post `
  -Headers @{ Authorization = "Bearer $env:BOOKY_ADMIN_TOKEN" } `
  "http://localhost:8080/admin/runs/daily-close?date=2026-04-01"
```

Replace the date with the relevant Stockholm accounting day.

Expected result:

1. pending facts are aggregated
2. Bokio accounts are validated
3. Bokio fiscal year is checked
4. Bokio journal is created
5. PDF attachment is uploaded

### 5. Verify in Bokio test company

In Bokio test environment, verify:

1. one journal entry exists for the posting date
2. title matches `Stripe dagsverifikat YYYY-MM-DD`
3. journal is balanced in SEK
4. the PDF attachment exists
5. the lines use the expected configured accounts

## Recommended Test Sequence

Run these tests in order.

### Test 1. Basic startup

1. start Postgres
2. start `booky`
3. verify `/healthz`

Expected:

1. migrations applied
2. no startup validation errors

### Test 2. Charge ingestion

1. start `stripe listen`
2. run `stripe trigger charge.succeeded`

Expected:

1. one row in `stripe_webhook_events`
2. one charge snapshot
3. one balance transaction snapshot
4. accounting facts for:
   1. receivable debit
   2. revenue credit
   3. VAT credit when configured
   4. fee expense debit
   5. fee balance credit
   6. reverse-charge VAT on fee
   7. available-on transfer rows

### Test 3. Refund ingestion

1. run `stripe trigger charge.refunded`

Expected:

1. refund facts are created
2. refund reverses receivable, revenue, and VAT in SEK

### Test 4. Dispute ingestion

1. run `stripe trigger charge.dispute.created`

Expected:

1. dispute facts are created
2. dispute debits dispute account and credits Stripe balance account or fallback OBS account

### Test 5. Payout ingestion

1. run `stripe trigger payout.paid`

Expected:

1. payout facts are created
2. bank account is debited
3. Stripe balance account is credited

### Test 6. Daily close to Bokio test company

1. confirm the Bokio test company fiscal year is open
2. confirm all configured accounts exist in Bokio
3. call `/admin/runs/daily-close`

Expected:

1. `posting_runs` row created
2. `bokio_journals` row created
3. `accounting_facts.status` changes to `posted`
4. attachment uploaded to Bokio

## Production Mode

Production should run the same binary, with different config and secrets.

### 1. Build binary

```powershell
go build -o bin/booky.exe ./cmd/booky
```

On Linux:

```bash
go build -o bin/booky ./cmd/booky
```

### 2. Production requirements

Use:

1. managed PostgreSQL with backups
2. TLS termination or private ingress in front of the app
3. secret manager for `STRIPE_API_KEY`, `STRIPE_WEBHOOK_SECRET`, and `BOKIO_TOKEN`
4. Bokio production company only after validating everything in test setup first

### 3. Production config changes

Typical production changes:

1. `app.env: prod`
2. `posting.scheduler_enabled: true`
3. `postgres.dsn` points to managed DB
4. Bokio company and token point to production tenant
5. Stripe webhook secret comes from the actual production webhook endpoint

### 4. Example production environment variables

PowerShell:

```powershell
$env:BOOKY_CONFIG = "C:\booky\config\booky.prod.yaml"
$env:STRIPE_API_KEY = "sk_live_..."
$env:STRIPE_WEBHOOK_SECRET = "whsec_live_..."
$env:BOKIO_TOKEN = "..."
```

Linux systemd style environment file would contain the same values.

### 5. Run production binary

Windows:

```powershell
bin\booky.exe
```

Linux:

```bash
./bin/booky
```

### 6. Production deployment checklist

Before enabling live traffic:

1. verify `/healthz`
2. verify DB migrations completed
3. verify Bokio account mapping against target company
4. verify Bokio fiscal year is open
5. verify Stripe webhook endpoint points to the deployed service
6. verify one manual Stripe test event reaches production service
7. keep scheduler disabled until webhook ingestion is confirmed
8. enable scheduler only after a successful controlled manual daily-close test

## Stripe Test Mode to Bokio Test Company Flow

This is the recommended non-production acceptance path.

### Setup

1. Stripe account in test mode
2. Bokio test company with test API token
3. `booky` running locally or in a non-production environment
4. `posting.scheduler_enabled: false`

### Execution

1. start `booky`
2. start `stripe listen --forward-to localhost:8080/webhooks/stripe`
3. trigger Stripe test events
4. inspect pending rows in PostgreSQL
5. run `/admin/runs/daily-close`
6. verify journal and PDF in Bokio test company

### Exit criteria

Accept the environment only when:

1. sale postings use the right revenue and VAT accounts by market
2. fees use the configured fee and reverse-charge VAT accounts
3. payouts hit the bank and Stripe balance accounts correctly
4. refunds and disputes create expected reversing entries
5. unknown cases route to OBS account, not silent failure
6. Bokio attachment lists the included Stripe facts

## Common Problems

### `initialize app: ... bokio.company_id is required`

`config/booky.yaml` is missing required values.

### `stripe webhook signature mismatch`

You are likely using the wrong webhook secret.

Use the secret printed by:

```powershell
stripe listen --forward-to localhost:8080/webhooks/stripe
```

### `bokio chart of accounts missing accounts`

One or more configured account numbers do not exist in the Bokio company.

Update `config/booky.yaml` to match the Bokio test company chart of accounts.

### `no open Bokio fiscal year covering YYYY-MM-DD`

The posting date is outside an open fiscal year in Bokio.

Use a valid date or open the correct fiscal year in the Bokio test company.

### daily-close creates no journal

Possible causes:

1. no pending facts exist for that date
2. facts were already posted
3. the webhook data landed on another accounting date than expected

Check:

```sql
SELECT source_group_id, posting_date, status
FROM accounting_facts
ORDER BY posting_date DESC, created_at DESC;
```

## Current Implementation Notes

At the current stage of the codebase:

1. normal charge, refund, dispute-created, and payout-paid flows are implemented first
2. scheduler is optional and should stay off during initial testing
3. reversal and repost tooling is planned but not yet exposed as an API endpoint
4. PDF output is intentionally plain and audit-focused

## Suggested Next Operational Improvements

Before real production use, add:

1. structured request IDs and correlation IDs
2. metrics and alerting
3. replay endpoint for one Stripe event or one posting date
4. stronger Stripe fixture-based integration tests
5. deployment unit files or container image docs
