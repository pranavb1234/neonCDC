# CDC Lite self-hosted installation

CDC Lite watches changes in a PostgreSQL database, stores events safely in Redis, and sends them to configured webhooks. This package starts CDC Lite and Redis locally. It does **not** start PostgreSQL: you connect your own source database through CDC Lite.

## Before you start

Install Docker Desktop with Docker Compose (Windows/macOS), or Docker Engine with the Docker Compose plugin (Linux). You do not need Go, Node.js, or Redis installed on your computer.

The CDC Lite image must be published to a container registry before customers can pull it. Ask the CDC Lite publisher for the exact image reference; this ZIP intentionally does not invent one.

## Install

1. Download and extract the CDC Lite Docker Compose ZIP.
2. Open PowerShell in the extracted `CDC-Lite-Docker-Compose-<version>` folder.
3. Create your local configuration file:

   ```powershell
   Copy-Item .env.example .env
   ```

4. Edit `.env`. Set `CDC_LITE_IMAGE` to the exact published image reference you were given. Keep the remaining values private.
5. Generate an encryption key:

   ```powershell
   $bytes = New-Object byte[] 32
   [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
   [Convert]::ToBase64String($bytes)
   ```

   Put that output in `APP_ENCRYPTION_KEY`. Keep this value forever. It encrypts saved database connection strings and webhook secrets; changing it later prevents CDC Lite from reading old saved values.
6. Set `ADMIN_EMAIL` and `ADMIN_PASSWORD`. Use a unique password of at least 12 characters.
7. Start CDC Lite:

   ```powershell
   docker compose up -d
   ```

8. Open the web console at [http://localhost:3000](http://localhost:3000).

Redis is private to Docker and is not exposed to your computer. Customer metadata is stored in the `cdc-lite-data` Docker volume. Redis stream data is stored in `cdc-lite-redis-data`.

## Administrator credentials

The first startup creates the administrator from the values in `.env`; CDC Lite stores only a password hash. It never prints the password in logs.

Keep the initial email and password in your password manager. After the first successful startup, remove the `ADMIN_EMAIL` and `ADMIN_PASSWORD` lines from `.env`; the stored administrator remains available. The current server release creates this administrator record for the upcoming login UI.

To change the stored administrator password, run this from the extracted package folder. The password is read from standard input and is never passed as a command-line argument:

```powershell
$securePassword = Read-Host "New administrator password" -AsSecureString
$pointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($securePassword)
try {
  [Runtime.InteropServices.Marshal]::PtrToStringBSTR($pointer) |
    docker compose run --rm -T cdc-lite-server admin-password --stdin
} finally {
  [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($pointer)
  Remove-Variable securePassword
}
```

## Everyday commands

Check status and logs:

```powershell
docker compose ps
docker compose logs -f cdc-lite-server
docker compose logs -f redis
```

Stop without deleting customer data:

```powershell
docker compose down
```

Start again:

```powershell
docker compose up -d
```

If startup fails, run `docker compose logs cdc-lite-server`. Check that `CDC_LITE_IMAGE` is a real published image reference, `APP_ENCRYPTION_KEY` is a base64 value for 32 random bytes, and the first startup has both administrator variables.

## Backup and restore

Stop CDC Lite before copying its volumes:

```powershell
docker compose down
$backupPath = (Get-Location).Path
docker run --rm -v cdc-lite-data:/data -v "${backupPath}:/backup" alpine tar czf /backup/cdc-lite-data-backup.tar.gz -C /data .
docker run --rm -v cdc-lite-redis-data:/data -v "${backupPath}:/backup" alpine tar czf /backup/cdc-lite-redis-data-backup.tar.gz -C /data .
```

To restore, keep the same `APP_ENCRYPTION_KEY`, stop the application, then run:

```powershell
docker compose down
$backupPath = (Get-Location).Path
docker run --rm -v cdc-lite-data:/data -v "${backupPath}:/backup" alpine sh -c "rm -rf /data/* && tar xzf /backup/cdc-lite-data-backup.tar.gz -C /data"
docker run --rm -v cdc-lite-redis-data:/data -v "${backupPath}:/backup" alpine sh -c "rm -rf /data/* && tar xzf /backup/cdc-lite-redis-data-backup.tar.gz -C /data"
docker compose up -d
```

## Update CDC Lite

Change `CDC_LITE_IMAGE` in `.env` to the newer published image reference, then run:

```powershell
docker compose pull
docker compose up -d
```

Do not use `-v` during an update. Docker keeps both persistent volumes.

## Uninstall

To remove containers and networks but keep customer data for a later reinstall:

```powershell
docker compose down
```

Only when you intentionally want to permanently delete all saved CDC Lite metadata and Redis events:

```powershell
docker compose down -v
```
