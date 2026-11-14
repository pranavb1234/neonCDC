# CDC Lite

A small local demo of PostgreSQL Change Data Capture: a Go connector reads committed row changes from Neon PostgreSQL, stores JSON events in Redis Streams, a Go worker posts them to a webhook, and a Next.js dashboard displays them.

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
