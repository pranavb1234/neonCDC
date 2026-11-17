# CDC Lite

A small local demo of PostgreSQL Change Data Capture: a Go connector reads committed row changes from Neon PostgreSQL, stores JSON events in Redis Streams, a Go worker posts them to a webhook, and a Next.js dashboard displays them.

## Resume highlights

- Built **CDC Lite**, a self-hosted PostgreSQL Change Data Capture platform using **Go, PostgreSQL logical replication (`pgoutput`), Redis Streams, Next.js, SQLite, and Docker Compose**.
- Implemented a Go CDC connector that decodes committed `INSERT`, `UPDATE`, and `DELETE` changes, buffers rows until PostgreSQL `Commit`, and publishes to Redis before acknowledging the LSN.
- Designed an **at-least-once delivery** workflow with stable `commit_lsn:sequence` event IDs, allowing webhook consumers to deduplicate events after retry or recovery.
- Developed HMAC-SHA256 signed webhook delivery with timestamped headers, Redis consumer groups, bounded delivery history, and evidence-based sink health states.
- Built a Next.js operational console for database connection testing, safe schema/table browsing, row previews, pipeline management, live events over SSE, and webhook delivery inspection.
- Secured customer metadata with SQLite foreign keys, encrypted PostgreSQL URLs and webhook secrets, bcrypt password hashes, error redaction, and SSRF protections for webhook targets.
- Packaged the server as a non-root Docker image and Docker Compose release with persistent volumes, health checks, graceful shutdown, and an installable customer ZIP.
- Created a separate Fumadocs landing and documentation website for technical onboarding and product documentation.

**One-line project description:** CDC Lite is a self-hosted event-driven PostgreSQL CDC platform that streams committed database changes through Redis Streams to HMAC-signed webhooks, with a Go backend and Next.js monitoring dashboard.

## Engineering documentation

These documents are written for portfolio review and interview preparation:

- [Architecture and repository map](docs/ARCHITECTURE.md)
- [Component and function reference](docs/COMPONENT_REFERENCE.md)
- [End-to-end application flows](docs/FLOWS.md)
- [Interview walkthrough](docs/INTERVIEW_GUIDE.md)
- [System design](docs/SYSTEM_DESIGN.md)
- [How to scale beyond this student project](docs/SCALING_TO_MILLIONS.md)
- [Security, trade-offs, and known limitations](docs/SECURITY_AND_LIMITATIONS.md)

## Public website and documentation

The customer console in `web/` is separate from the public landing and documentation site in `website/`. The website uses Fumadocs, with its landing page at `/` and documentation at `/docs`.

```powershell
Set-Location website
npm install
npm run dev
```

Open `http://localhost:3000` for the landing page and `http://localhost:3000/docs` for the Fumadocs guide. Run the customer console separately from `web/`; it also defaults to port 3000, so choose a different port when both are running.

## What is included now

This first step provides the local Redis service and configuration scaffold. The connector, delivery worker, dashboard, and sample receiver will be added in later steps.

## Setup

1. Create a Neon project and database.
2. In Neon project settings, enable logical replication.
3. Copy the **direct, unpooled** connection string — not the `-pooler` connection string — into `SOURCE_DB_URL`.
4. Copy `.env.example` to `.env`, add your local values, and keep `.env` out of source control.

Neon must use SSL; the example connection string includes `sslmode=require`.

## Create sample tables in Neon

Open your Neon project, choose the database, and open **SQL Editor**. Paste and run [`sample/source-init.sql`](sample/source-init.sql). Alternatively, connect to Neon using the direct connection string and run that file with any PostgreSQL client.

The script creates `users` and `orders`, adds a few rows, and sets `REPLICA IDENTITY FULL` on both tables so later CDC events include the complete row identity.

After running it, use the Neon SQL Editor to create changes manually:

```sql
INSERT INTO users (name, email) VALUES ('Linus Torvalds', 'linus@example.com');
UPDATE orders SET status = 'shipped' WHERE id = 1;
DELETE FROM users WHERE id = 2;
```

These statements run directly against Neon. No local PostgreSQL container is used.

## Run the connector

Install Go, start local Redis, then run the connector from the `worker` directory. The connector reads `SOURCE_DB_URL` and `REDIS_URL` only from environment variables, checks both connections at startup, and waits until you press `Ctrl+C`.

PowerShell:

```powershell
make up
Get-Content .env | ForEach-Object { if ($_ -match '^([^#][^=]*)=(.*)$') { [Environment]::SetEnvironmentVariable($matches[1], $matches[2], 'Process') } }
Set-Location worker
go run ./cmd/connector
```

`SOURCE_DB_URL` must be the Neon direct endpoint, never the `-pooler` endpoint, and must include a secure `sslmode`.

## Enable CDC reading

Enable **Logical Replication** in Neon project settings before starting the connector. The connector checks that `wal_level` is `logical`, confirms the connected role has `REPLICATION`, creates `cdc_lite_publication` for the sample `users` and `orders` tables if needed, and creates or reuses `cdc_lite_slot` with `pgoutput`.

You can override these names with `PUBLICATION_NAME` and `REPLICATION_SLOT`. Use a role created through Neon Console, CLI, or API: Neon documents that SQL-created roles cannot be granted the required replication privilege.

## Run the delivery worker

Set `WEBHOOK_URL` and `WEBHOOK_SECRET` in `.env`. The delivery worker signs each raw JSON payload as `timestamp + "." + body` with HMAC-SHA256 and only acknowledges Redis messages after a successful 2xx webhook response.

For the included local receiver, open a separate PowerShell window:

```powershell
Set-Location sample/receiver
go run .
```

It listens at the default `WEBHOOK_URL`, `http://localhost:8080/webhook`, verifies the HMAC signature, prints the event ID and operation, and returns HTTP 200. Invalid or modified requests return HTTP 401.

```powershell
Set-Location worker
go run ./cmd/delivery
```

## Run the dashboard

The dashboard reads the same local Redis stream and can send a signed webhook test. Create `web/.env.local` with only the dashboard values from your root `.env`:

```powershell
Get-Content .env | Where-Object { $_ -match '^(REDIS_URL|WEBHOOK_URL|WEBHOOK_SECRET)=' } | Set-Content web/.env.local
Set-Location web
npm install
npm run dev
```

Open `http://localhost:3000`. The Live Events page receives new Redis Stream entries over Server-Sent Events; it is a local demo and does not require login.

## Initialize self-hosted metadata

The future self-hosted server stores its administrator, saved connection metadata, pipelines, sinks, and delivery history in SQLite. The database is stored in the `cdc-lite-data` Docker volume; Redis remains the event buffer.

Copy the `APP_ENCRYPTION_KEY` placeholder from `metadata.env.example` into your local `.env`, replacing it with a base64-encoded random 32-byte key. Do not change this key after saving connections or sink secrets, because existing encrypted values would become unreadable.

Initialize the schema and the one administrator without putting the password on the command line:

```powershell
$securePassword = Read-Host "Administrator password" -AsSecureString
$pointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($securePassword)
try {
  [Runtime.InteropServices.Marshal]::PtrToStringBSTR($pointer) | docker compose --profile tools run --rm -T metadata init --admin-email admin@example.com --admin-password-stdin
} finally {
  [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($pointer)
  Remove-Variable securePassword
}
```

To apply migrations later or inspect the persisted schema:

```powershell
docker compose --profile tools run --rm metadata migrate
docker compose --profile tools run --rm metadata schema
```

The metadata CLI is intentionally a one-shot tool in this step; it does not yet start CDC pipelines or provide the planned self-hosted UI.

## Manage saved PostgreSQL databases

In the packaged server console, open **Databases** and add a display name plus a PostgreSQL connection string. CDC Lite tests ordinary SQL connectivity and then checks `wal_level=logical` and the current role's `REPLICATION` permission separately. A successful SQL login alone is not treated as CDC-ready.

For remote sources, provide `sslmode=require`, `sslmode=verify-ca`, or `sslmode=verify-full`. Use a direct, non-pooled endpoint for logical replication. The connection string is encrypted with `APP_ENCRYPTION_KEY` before it is stored, is never returned by the API, and is not logged. Keep that key stable across restarts and restore it with the metadata volume backup; otherwise saved connections cannot be decrypted.

Saving a database never starts a CDC pipeline automatically.

Selecting **Browse** on a saved database opens a read-only inspection view. It lists non-system schemas and base tables, then shows column metadata, primary-key columns, replica identity, and at most 25 preview rows. CDC Lite quotes selected schema/table identifiers, uses a short read-only transaction, and never accepts browser-provided SQL. It does not modify source schemas or table data.

## Create a CDC pipeline

From a saved database's **Browse** screen, select a schema and one or more tables, then choose **Create Pipeline**. CDC Lite validates that the tables still exist and verifies logical replication prerequisites again before it creates a unique publication and replication slot. It does not change table replica identity; tables without a primary key or `REPLICA IDENTITY FULL` remain clearly flagged in the table inspector.

The first customer release supports **one active pipeline per CDC Lite installation**. Additional pipelines may be saved but must remain stopped until the active one is stopped. Stopping a pipeline preserves its publication and slot for a restart. Delete is available only after a pipeline is stopped, then removes only that pipeline's deterministically named publication and slot.

## Add a Webhook sink

Open **Sinks**, select **Create Sink**, choose the source pipeline, then enter an HTTPS webhook URL. You may enter a signing secret or leave it blank for CDC Lite to generate one. A generated secret is shown only once; save it in the receiving application before closing the message. Secrets are encrypted in the metadata volume and are never returned in the sink list.

Each sink receives events from its selected pipeline only. CDC Lite sends the exact JSON event body with `X-CDC-Event-Id`, `X-CDC-Timestamp`, and `X-CDC-Signature: v1=<HMAC-SHA256>` headers. Redirects and loopback, private, link-local, and cloud-metadata destinations are blocked; hostnames are checked again immediately before connection.

Delivery is sequential. A failed request remains pending in Redis and is retried after a short delay; there is no retry schedule or dead-letter queue in this first release. Duplicate webhook deliveries remain possible.

The sink overview derives its health from actual recorded deliveries: **Healthy** means the newest attempt succeeded; **Degraded** means the newest attempt failed after earlier success; **Failing** means at least three recent attempts all failed; **Paused** means the sink is disabled; and **Unknown** means it has never been attempted. It retains the newest 100 attempt records per sink and displays the newest 20. History includes only event ID, time, outcome, HTTP status, and a redacted error summary — never event payloads or secrets.

### End-to-end webhook demonstration

1. Open a unique endpoint at [Webhook.site](https://webhook.site/) and copy its HTTPS URL. (The packaged server deliberately rejects `localhost` and other private destinations to prevent SSRF.)
2. In **Sinks**, create an enabled Webhook sink for the running pipeline, using the Webhook.site URL. Keep the generated signing secret if you want to verify the signature independently.
3. Click **Send Test Event**. The sink should show a successful delivery and the request should appear in Webhook.site with the `X-CDC-Event-Id`, `X-CDC-Timestamp`, and `X-CDC-Signature` headers.
4. Start the selected pipeline, then make an `INSERT`, `UPDATE`, or `DELETE` on one of its selected source tables.
5. Refresh the sink overview. A delivery attempt for the CDC event ID should appear. The receiver must treat that ID as an idempotency key because Redis/CDC recovery can deliver an event more than once.

For the included local Go receiver, expose `sample/receiver` through a public HTTPS tunnel and use the resulting public URL; do not weaken the production SSRF protection merely to target `localhost`.

## Create a customer Docker Compose ZIP

Build the customer server image from the repository root:

```powershell
docker build -t cdc-lite-server:local .
```

Before customers can use the ZIP, publish that image to a container registry and give them its exact image reference. The Compose package intentionally requires `CDC_LITE_IMAGE` instead of guessing a registry or image name.

Create a ZIP containing only the Docker Compose package files:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\package-release.ps1 -Version 0.1.0
```

The command prints the ZIP path and SHA-256 checksum. Verify its contents:

```powershell
Expand-Archive -LiteralPath .\dist\CDC-Lite-Docker-Compose-0.1.0.zip -DestinationPath .\dist\verify -Force
Get-ChildItem -Recurse -File .\dist\verify
Get-FileHash -Algorithm SHA256 .\dist\CDC-Lite-Docker-Compose-0.1.0.zip
```

The ZIP contains only `docker-compose.yml`, `.env.example`, and `README.md` inside its top-level release folder.

## Run Redis

```sh
cp .env.example .env
make up
```

Redis will be available at `redis://localhost:6379`. No local PostgreSQL container or installation is needed: Neon is the source database.

Useful commands:

```sh
make logs   # follow Redis logs
make down   # stop local services
make reset  # remove Redis data and start it again
```
