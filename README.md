# booky

`booky` turns Stripe activity into bookkeeping-ready data for Bokio.

It receives Stripe webhooks, stores the data in PostgreSQL, groups transactions by accounting date, creates Bokio journal entries, uploads a PDF attachment for each posting, and can also prepare filing exports such as OSS Union and periodic summary reports.

The project is built for teams that want Stripe payments to flow into Bokio in a controlled, auditable way instead of being handled manually.

## What it can do

- Receive Stripe webhooks
- Store raw Stripe events and normalized accounting data in PostgreSQL
- Handle core Stripe accounting flows such as sales, refunds, disputes, payouts, and fees
- Post daily journal entries to Bokio
- Upload a PDF attachment together with the Bokio journal
- Generate filing runs for OSS Union and periodic summary workflows
- Send notifications for important accounting or filing issues when Resend is enabled

## What you need

Before you start, make sure you have:

- Go `1.26+`
- Docker or another way to run PostgreSQL
- A PostgreSQL database
- A Stripe account and webhook secret
- A Bokio company ID and API token

For local development, Stripe CLI is also useful so you can forward webhooks to your machine.

## Configuration

The app reads its config from `config/booky.yaml` by default.

Start by copying the example file:

```bash
cp config/booky.example.yaml config/booky.yaml
```

At a minimum, you should set:

- PostgreSQL connection string
- Stripe API key
- Stripe webhook secret
- Bokio company ID
- Bokio token
- Bokio account mappings under `accounts`
- An admin bearer token if you want to use the protected admin endpoints

You can point to another config file with:

```bash
export BOOKY_CONFIG="config/booky.dev.yaml"
```

## Development setup

### 1. Start PostgreSQL

This repository includes a local PostgreSQL service:

```bash
docker compose up -d postgres
```

### 2. Prepare your config and secrets

Example local environment variables:

```bash
export BOOKY_POSTGRES_DSN="postgresql://booky:booky@localhost:5432/booky?sslmode=disable"
export BOOKY_ADMIN_TOKEN="change-me"
export STRIPE_API_KEY="sk_test_..."
export STRIPE_WEBHOOK_SECRET="whsec_..."
export BOKIO_TOKEN="..."
```

Then update `config/booky.yaml` with the correct Bokio company ID and account mappings.

For local development, a good default is:

- `app.env: dev`
- `posting.scheduler_enabled: false`
- `admin.enabled: true`
- `filings.enabled: false` unless you are actively working on filing flows

### 3. Run the app

```bash
go run ./cmd/booky
```

The service listens on port `8080` by default.

For live reload during development, use the included Air config:

```bash
air
```

It rebuilds `./cmd/booky` and restarts the app whenever Go or YAML files change.

### 4. Check that it started

```bash
curl http://localhost:8080/healthz
```

Expected response:

```json
{"status":"ok"}
```

### 5. Forward Stripe webhooks

For end-to-end local testing:

```bash
stripe listen --forward-to localhost:8080/webhooks/stripe
```

Use the webhook secret printed by Stripe CLI as your local `STRIPE_WEBHOOK_SECRET`.

### 6. Trigger a daily close manually

When you want `booky` to create the Bokio journal for a specific accounting date:

```bash
curl -X POST \
  -H "Authorization: Bearer $BOOKY_ADMIN_TOKEN" \
  "http://localhost:8080/admin/runs/daily-close?date=2026-04-01"
```

## Production setup

Production uses the same application, but with production config, production secrets, and a real PostgreSQL database.

Recommended production setup:

- Managed PostgreSQL with backups
- Secure secret storage for Stripe and Bokio credentials
- A stable public HTTPS endpoint for Stripe webhooks
- `posting.scheduler_enabled: true` only after you have verified webhook ingestion and a successful manual daily close

### Option 1: Run the compiled binary

Build:

```bash
go build -o bin/booky ./cmd/booky
```

Run:

```bash
./bin/booky
```

Typical production environment variables:

```bash
export BOOKY_CONFIG="/etc/booky/booky.prod.yaml"
export STRIPE_API_KEY="sk_live_..."
export STRIPE_WEBHOOK_SECRET="whsec_live_..."
export BOKIO_TOKEN="..."
```

### Option 2: Run with Docker

Build the image:

```bash
docker build -t booky .
```

Run it with your production config and environment variables mounted into the container. The container expects the config at `/app/config/booky.yaml` unless you override `BOOKY_CONFIG`.

## Production checklist

Before sending real traffic to `booky`, confirm:

- `/healthz` returns `ok`
- database migrations run successfully
- the Bokio company ID and account mappings are correct
- the Bokio fiscal year is open for the posting dates you expect to use
- Stripe points to your deployed `/webhooks/stripe` endpoint
- one controlled test event reaches the service successfully
- manual daily close works before the scheduler is enabled

## Main endpoints

- `GET /healthz` for service health
- `POST /webhooks/stripe` for Stripe webhook intake
- `POST /admin/runs/daily-close?date=YYYY-MM-DD` to post one accounting day to Bokio
- `POST /admin/runs/filing?kind=...&period=...` to run a filing export
- `GET /admin/filings?kind=...&period=...` to inspect filing status
- `POST /admin/filings/mark-submitted?kind=...&period=...` to stop reminders after submission
- `GET /admin/bokio/check` to verify Bokio connectivity and configuration

Admin endpoints require the configured bearer token when `admin.enabled` is turned on.

### Endpoint query parameter examples

#### `GET /healthz`

No query parameters.

```bash
curl "http://localhost:8080/healthz"
```

#### `POST /webhooks/stripe`

No query parameters. Stripe sends the payload in the request body and the signature in the `Stripe-Signature` header.

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -H "Stripe-Signature: $STRIPE_SIGNATURE" \
  --data @event.json \
  "http://localhost:8080/webhooks/stripe"
```

#### `POST /admin/runs/daily-close`

Query parameters:

- `date` optional, format `YYYY-MM-DD`

If `date` is omitted, `booky` uses the current date in the configured local timezone.

```bash
curl -X POST \
  -H "Authorization: Bearer $BOOKY_ADMIN_TOKEN" \
  "http://localhost:8080/admin/runs/daily-close"
```

```bash
curl -X POST \
  -H "Authorization: Bearer $BOOKY_ADMIN_TOKEN" \
  "http://localhost:8080/admin/runs/daily-close?date=2026-03-31"
```

#### `POST /admin/runs/filing`

Query parameters:

- `kind` required, for example `oss_union` or `periodic_summary`
- `period` required

Example period formats used by the service:

- OSS Union quarter: `2026-Q1`
- Periodisk sammanstallning month: `2026-03`

```bash
curl -X POST \
  -H "Authorization: Bearer $BOOKY_ADMIN_TOKEN" \
  "http://localhost:8080/admin/runs/filing?kind=oss_union&period=2026-Q1"
```

```bash
curl -X POST \
  -H "Authorization: Bearer $BOOKY_ADMIN_TOKEN" \
  "http://localhost:8080/admin/runs/filing?kind=periodic_summary&period=2026-03"
```

#### `GET /admin/filings`

Query parameters:

- `kind` required
- `period` required

```bash
curl \
  -H "Authorization: Bearer $BOOKY_ADMIN_TOKEN" \
  "http://localhost:8080/admin/filings?kind=oss_union&period=2026-Q1"
```

```bash
curl \
  -H "Authorization: Bearer $BOOKY_ADMIN_TOKEN" \
  "http://localhost:8080/admin/filings?kind=periodic_summary&period=2026-03"
```

#### `POST /admin/filings/mark-submitted`

Query parameters:

- `kind` required
- `period` required
- `submitted_at` optional, must be RFC3339 like `2026-04-02T09:15:00Z`

If `submitted_at` is omitted, `booky` uses the current time.

```bash
curl -X POST \
  -H "Authorization: Bearer $BOOKY_ADMIN_TOKEN" \
  "http://localhost:8080/admin/filings/mark-submitted?kind=oss_union&period=2026-Q1"
```

```bash
curl -X POST \
  -H "Authorization: Bearer $BOOKY_ADMIN_TOKEN" \
  "http://localhost:8080/admin/filings/mark-submitted?kind=periodic_summary&period=2026-03&submitted_at=2026-04-02T09:15:00Z"
```

#### `GET /admin/bokio/check`

No query parameters.

```bash
curl \
  -H "Authorization: Bearer $BOOKY_ADMIN_TOKEN" \
  "http://localhost:8080/admin/bokio/check"
```

## Filings for Skatteverket

`booky` can generate filing drafts for Skatteverket in the formats expected for:

- OSS Union
- Periodisk sammanställning

When filings are enabled, `booky` prepares period-based export files from the stored Stripe accounting data and emails them to the recipients in `filings.email_to`.

The send timing is controlled by:

- `filings.lead_time_days`, which decides how many days before the filing deadline the draft should first be sent
- `filings.send_time_local`, which decides the local time of day for that first send

In practice, that means:

- OSS Union drafts are prepared for the quarter and aimed at the Skatteverket deadline at the end of the following month
- Periodisk sammanställning drafts are prepared per month and aimed at the Skatteverket deadline on the 25th of the following month

Each draft email includes the generated file as an attachment. If there is no OSS-ready data for a period, `booky` can still send a nil-return reminder instead of a file so the team knows a filing may still be due.

After the filing has been submitted to Skatteverket, mark that period as submitted through the admin endpoint so `booky` stops sending reminders for it:

```bash
curl -X POST \
  -H "Authorization: Bearer $BOOKY_ADMIN_TOKEN" \
  "http://localhost:8080/admin/filings/mark-submitted?kind=oss_union&period=2026-Q1"
```

## Notes

- Keep `scheduler_enabled` off until you trust the environment and mappings.
- Most production issues come from wrong webhook secrets, missing Bokio accounts, or posting into a closed fiscal year.
- `booky` is strict on accounting safety. If data is incomplete or unclear, it prefers review over guessing.
