# CDC Lite self-hosted package

This package starts only CDC Lite and Redis. It does not start a PostgreSQL source database; customers connect their own PostgreSQL database through the future server UI/API.

## Prerequisites

Install Docker Desktop (Windows/macOS) or Docker Engine plus the Docker Compose plugin (Linux). No Go, Node.js, or Redis installation is needed.

## First startup

1. Download and extract this `docker-compose` folder.
2. Copy `.env.example` to `.env`.
3. Set `ADMIN_EMAIL` and a unique `ADMIN_PASSWORD` of at least 12 characters. These values create the local administrator only on the first startup; later changes do not reset its password.
4. Generate a persistent encryption key and put it in `APP_ENCRYPTION_KEY`. In PowerShell:

   ```powershell
   $bytes = New-Object byte[] 32
   [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
   [Convert]::ToBase64String($bytes)
   ```

   Do not rotate this key after saved connection strings or sink secrets exist, or those encrypted values cannot be read.
5. Set `CDC_LITE_IMAGE` to the published image tag supplied by CDC Lite. For a local repository build, use `cdc-lite-server:local`.
6. Start the application:

   ```sh
   docker compose up -d
   ```
7. Open `http://localhost:3000` (or your `CDC_LITE_PORT`).

Redis is internal-only and has no host port mapping. Metadata and Redis data persist in Docker volumes named `cdc-lite-data` and `redis-data`.

## Build a local image

From the repository root (not this package folder):

```sh
docker build -t cdc-lite-server:local .
```

Then set `CDC_LITE_IMAGE=cdc-lite-server:local` in this package's `.env` before starting Compose.

## Operations

```sh
docker compose ps
docker compose logs -f cdc-lite-server
docker compose logs -f redis
docker compose down
docker compose pull && docker compose up -d
```

Check server health:

```sh
curl http://localhost:3000/healthz
```

Do not run `docker compose down -v` unless you intentionally want to delete saved application metadata and queued Redis events.
