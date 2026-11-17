# Walflow component and function reference

This document is a code-level map. It explains what calls what in the current implementation, without pretending every helper is a separate service.

## 1. Server startup: `worker/cmd/server/main.go`

```text
main()
  ├─ logging.New()
  ├─ resetAdminPassword()           [only for `admin-password --stdin`]
  └─ run(logger)
       ├─ signal.NotifyContext()    [SIGINT/SIGTERM root context]
       ├─ metadata.Open()
       │    ├─ newEncryptor()
       │    ├─ opens SQLite
       │    └─ enables foreign keys, WAL, busy timeout
       ├─ store.Migrate()
       ├─ store.MarkInterruptedPipelinesStopped()
       ├─ bootstrapAdmin()
       │    ├─ store.HasAdministrator()
       │    └─ store.BootstrapAdministrator()
       ├─ redis.ParseURL() + redis.NewClient() + Ping()
       ├─ go application.deliveryLoop(ctx)
       ├─ application.routes()
       └─ http.Server.ListenAndServe()
```

`main()` only decides whether to run normal server mode or password-reset mode. `run()` constructs the process dependencies once. `bootstrapAdmin()` creates an administrator only when no administrator row exists; it never reads a password from SQLite because the stored value is a bcrypt hash.

On shutdown, the signal cancels the root context. That cancellation reaches the replication goroutine and delivery loop. The HTTP server receives `Shutdown` with a 15-second deadline.

## 2. HTTP routing and browser-to-server calls

`routes()` uses Go 1.22 method/path patterns. It is the single map for packaged API endpoints.

| Browser action | Client component | HTTP route | Go handler | Main store/runtime calls |
| --- | --- | --- | --- | --- |
| List databases | `Databases.load` | `GET /api/databases` | `listDatabases` | `ListSourceConnections` |
| Test entered URL | `Databases.test` | `POST /api/databases/test` | `testDatabaseInput` | `testPostgres` |
| Save database | `Databases.save` | `POST /api/databases` | `createDatabase` | `testPostgres` → `CreateSourceConnection` |
| Re-test saved source | `savedTest` | `POST /api/databases/{id}/test` | `testSavedDatabase` | `SourceConnectionURL` → `testPostgres` → `SetSourceConnectionTest` |
| Browse schemas | `DatabaseBrowser` effect | `GET .../schemas` | `listSchemas` | `readOnlyDatabase` |
| Browse tables | `chooseSchema` | `GET .../tables` | `listTables` | `readOnlyDatabase` |
| Inspect a table | `chooseTable` | `GET .../{table}` | `tableDetails` | `readOnlyDatabase` |
| Preview data | `loadPreview` | `GET .../preview` | `tablePreview` | `readOnlyDatabase` |
| Create pipeline | `createPipeline` | `POST .../pipelines` | `createPipeline` | `validatePipelineTables` → `pipelineNames` → `CreatePipeline` → `launchPipeline` |
| Start/stop/delete | `controlPipeline` | `/api/pipelines/{id}/...` | corresponding handler | `launchPipeline`, `pipelineManager.stop`, `cleanupPipeline` |
| List/create sinks | `Sinks.load/create` | `/api/sinks` | `listSinks/createSink` | `ListSinks`, `CreateWebhookSink` |
| Toggle sink | `Sinks.toggle` | `PATCH /api/sinks/{id}/enabled` | `setSinkEnabled` | `SetSinkEnabled` |
| Send test | `Sinks.sendTest` | `POST /api/sinks/{id}/test` | `sendSinkTest` | `Sink` → `deliverWebhook` → `RecordDeliveryAttempt` |
| Initial events | `EventList` effect | `GET /api/events` | `events` | Redis `XRevRangeN` → `decodeEvent` |
| Live updates | `EventSource` | `GET /api/events/stream` | `stream` | Redis `XRead` → `decodeEvent` → SSE write |

`decodeJSON` limits JSON body size and rejects unknown JSON fields. `pathID` validates URL IDs. `writeJSON` provides a consistent JSON response writer.

## 3. Database connection and inspection functions

`testPostgres(parent, connectionString)` is the gatekeeper for a new source. It creates a timeout context, parses the URL with pgx, requires a secure remote `sslmode`, opens a connection through `postgres.Connect`, pings it, then runs `SHOW wal_level` and reads `rolreplication` from `pg_roles` for the current user.

`readOnlyDatabase(request, work)` is the safe browsing wrapper. It gets the encrypted URL through `SourceConnectionURL`, opens PostgreSQL, starts a read-only transaction with a deadline, calls the passed work function, and closes/rolls back. `listSchemas`, `listTables`, `tableDetails`, and `tablePreview` are all built on it. Browser values are validated by `inspectableSchema`/`inspectableName`; table and schema names are used via safe identifier quoting rather than arbitrary SQL strings.

## 4. Pipeline lifecycle and function calls

```text
createPipeline handler
  ├─ decodeJSON[pipelineInput]()
  ├─ validatePipelineTables()
  │    └─ readOnlyDatabase() checks selected tables still exist
  ├─ pipelineNames(databaseID, name)
  ├─ store.CreatePipeline()
  └─ launchPipeline(pipelineID)
       ├─ checks one-active-pipeline limit in pipelineManager.running
       ├─ store.Pipeline()
       ├─ store.SourceConnectionURL()
       ├─ postgres.Connect()
       ├─ redis.NewPipelineStreamPublisher(redis, pipeline.ID)
       ├─ store.SetPipelineStatus(..., "running")
       └─ go replication.Run(jobContext, db, replication.Config, publisher, logger)
```

`pipelineManager.running` holds cancellation functions. `stopPipeline` calls `pipelineManager.stop`, then persists `stopped` in SQLite. Stop is intentionally idempotent: after a process restart, the in-memory map is empty but the database row can still be changed to stopped. `deletePipeline` checks both memory and persisted status, calls `cleanupPipeline` to drop only the pipeline's known publication and slot, then deletes metadata.

## 5. Logical replication package: `worker/internal/replication`

`replication.Run` is the CDC entry point called by `launchPipeline`.

```text
Run()
  ├─ verifyPrerequisites()          [wal_level + role replication flag]
  ├─ ensurePublication()            [create publication for selected tables if absent]
  ├─ replicationConnectionURL()     [adds replication=database]
  ├─ pgconn.Connect()
  ├─ ensureSlot()                   [creates pgoutput logical slot or reuses it]
  ├─ pglogrepl.StartReplication()
  └─ readMessages()
       ├─ ReceiveMessage()
       ├─ ParsePrimaryKeepaliveMessage() → SendStandbyStatusUpdate() when requested
       └─ ParseXLogData() → pglogrepl.Parse(WALData) → eventDecoder.Process()
            ├─ RelationMessage: cache relation ID, column names, OIDs, flags
            ├─ BeginMessage: start/reset transaction buffer
            ├─ Insert/Update/Delete: decode and append rowChange
            └─ CommitMessage: build stable Events
       ├─ publisher.Publish(events)  [Redis transaction pipeline]
       └─ acknowledgeCommit(LSN)     [only if Redis publish succeeded]
```

`eventDecoder` owns two pieces of state: a relation cache and a transaction buffer. `decodeTuple` maps tuple positions to relation column names. `decodeValue` handles the sample-project type set. `insert`, `update`, and `delete` build `rowChange` values. On commit, `commit()` produces final `Event` objects, sets the commit LSN, assigns `commitLSN:sequence`, and clears the buffer.

The key invariant lives in `readMessages`: `publisher.Publish` happens before `acknowledgeCommit`. If `Publish` returns an error, the function returns an error saying the LSN was not acknowledged. PostgreSQL can replay that committed change after reconnect.

## 6. Redis publisher: `worker/internal/redis`

`NewPipelineStreamPublisher` creates a small adapter that satisfies `replication.EventPublisher`. `Publish` JSON-marshals every decoded event, adds stream values with `streamValues`, queues all `XADD` operations in a Redis transaction pipeline, and executes them together. Values include `pipeline_id`, allowing the delivery loop to choose only relevant sinks.

`NewStreamPublisher` remains for the earlier standalone connector. `Connect` parses `REDIS_URL`, pings Redis, and returns a usable client.

## 7. Webhook delivery and sink operations

`run()` starts `deliveryLoop` in a goroutine. The loop creates/reuses `server-delivery`, then performs `XREADGROUP` against `cdc:events`.

```text
deliveryLoop()
  ├─ XREADGROUP(server-delivery, server-1, cdc:events)
  ├─ read pipeline_id + event payload
  ├─ store.EnabledSinksForPipeline(pipelineID)
  └─ for each enabled matching sink:
       deliverWebhook()
         ├─ validateWebhookURL()
         ├─ store.WebhookSigningSecret() [decrypts only now]
         ├─ HMAC(timestamp + "." + rawBody)
         ├─ webhookHTTPClient().Do(request)
         │    └─ safeWebhookDial() resolves DNS and blocks unsafe IPs
         └─ checks HTTP 2xx
       store.RecordDeliveryAttempt()
  └─ XACK only when all matching sinks succeeded
```

`validateWebhookURL` rejects malformed URLs, user-info URLs, unresolved hosts, and unsafe address classes. `webhookHTTPClient` disables proxies and redirects. Its custom `safeWebhookDial` resolves DNS again at dial time, preventing a host from passing validation and then DNS-rebinding to an internal IP. `webhookDeliveryError` retains HTTP status internally so the safe status can be written to history.

`sendSinkTest` builds a synthetic JSON event, calls the same `deliverWebhook` path used by real CDC events, then stores a success/failure attempt. It is not a separate fake delivery implementation.

## 8. Metadata store: `worker/internal/metadata`

`Open` configures SQLite. `Migrate` applies `0001_initial.sql` once and records it in `schema_migrations`. `crypto.go` implements the encryptor used by `CreateSourceConnection` and `CreateWebhookSink`.

The store methods form clear groups:

- Administrator: `BootstrapAdministrator`, `HasAdministrator`, `UpdateAdministratorPassword`.
- Source database: `CreateSourceConnection`, `SourceConnectionURL`, `ListSourceConnections`, `RenameSourceConnection`, `SetSourceConnectionTest`, `DeleteSourceConnection`.
- Pipeline: `CreatePipeline`, `ListPipelines`, `ListAllPipelines`, `Pipeline`, `SetPipelineStatus`, `DeletePipeline`, `MarkInterruptedPipelinesStopped`.
- Sink/delivery: `CreateWebhookSink`, `ListSinks`, `Sink`, `EnabledSinksForPipeline`, `WebhookSigningSecret`, `SetSinkEnabled`, `RecordDeliveryAttempt`.

`ListSinks` reads each sink's bounded history through `deliveryAttempts`, then calls `sinkHealth`. `RecordDeliveryAttempt` writes the attempt, updates last sink status, redacts the error through `safeError`, and trims old rows to 100 per sink in one SQL transaction.

## 9. Legacy local demo commands

`cmd/connector` and `cmd/delivery` are kept to demonstrate the original two-process learning architecture. The connector's `run` loads `config.Load`, opens PostgreSQL/Redis, and calls `replication.Run`. The delivery worker loads `config.LoadDelivery`, creates its `delivery` consumer group, reads fresh and pending messages, signs them through `sign`, then acknowledges on HTTP success. The packaged customer server consolidates those responsibilities into `cmd/server`.

## 10. File-by-file inventory

This table is intentionally exhaustive for project source files; generated files such as `.next`, `node_modules`, build output, and lockfile internals are excluded.

| File/group | What it contains or does |
| --- | --- |
| `worker/cmd/server/main.go` | Main product runtime: startup, routing, source inspection API, pipeline lifecycle, Redis delivery loop, signed webhook delivery, event/SSE APIs, static asset serving. |
| `worker/cmd/connector/main.go` | Standalone local connector entry point for the earlier demo architecture. |
| `worker/cmd/delivery/main.go` | Standalone local Redis-to-webhook worker with consumer-group and HMAC helpers. |
| `worker/cmd/delivery/main_test.go` | Focused tests for standalone delivery signing/message behavior. |
| `worker/cmd/metadata/main.go` | One-shot metadata commands: initialize/migrate/show schema/admin support. |
| `worker/Dockerfile.metadata` | Image recipe for the metadata command tool. |
| `worker/internal/config/config.go` | Validates environment variables and standalone connector/delivery configuration. |
| `worker/internal/logging/logging.go` | Creates the JSON structured logger used by Go executables. |
| `worker/internal/postgres/postgres.go` | Opens a pgx source connection with connection validation. |
| `worker/internal/replication/replication.go` | Owns publication/slot setup, replication connection, frame/message reading, keepalives, Redis-before-LSN acknowledgement. |
| `worker/internal/replication/event.go` | Holds event structures, relation cache, transaction buffer, tuple/type decoding, and stable event construction. |
| `worker/internal/replication/event_test.go` | Tests sample value decoding and the rule that changes are emitted only at commit. |
| `worker/internal/redis/redis.go` | Redis URL connection and `StreamPublisher` that writes committed event batches. |
| `worker/internal/redis/redis_test.go` | Tests stream fields including event metadata and pipeline ID. |
| `worker/internal/metadata/crypto.go` | AES-256-GCM encryptor construction and encrypt/decrypt helpers backed by the persistent app key. |
| `worker/internal/metadata/store.go` | SQLite open/migration plus all administrator, source, pipeline, sink, health, and delivery-history methods. |
| `worker/internal/metadata/migrations/0001_initial.sql` | Version-one relational schema, foreign keys, constraints, and indexes. |
| `worker/internal/metadata/store_test.go` | Tests encryption/redaction/history and evidence-based sink health. |
| `web/app/layout.tsx`, `globals.css` | Console shell and global dashboard styling. |
| `web/app/page.tsx`, `databases/page.tsx`, `database/page.tsx`, `events/page.tsx`, `sinks/page.tsx`, `webhook/page.tsx` | Thin App Router route files that render their feature component. |
| `web/components/databases.tsx` | Database list/add/test/save/rename/delete UI and its fetch calls. |
| `web/components/database-browser.tsx` | Read-only schema/table browser, previews, selected-table UI, and pipeline controls. |
| `web/components/event-list.tsx` | Fetches recent events, opens SSE, filters rows, and expands payload inspection. |
| `web/components/sinks.tsx` | Sink create/review form, enable/disable/test actions, health display, and delivery-history rendering. |
| `web/components/overview.tsx` | Dashboard metrics polling. |
| `web/components/webhook-settings.tsx` | Earlier standalone-demo webhook configuration/test presentation. |
| `web/components/nav.tsx` | Customer-console navigation. |
| `web/components/ui/{badge,button,card}.tsx` | Small reusable visual primitives only; not a large design system. |
| `web/app/api/*` | Earlier Next.js local-demo APIs for Redis reads/SSE and environment-configured webhook testing. Packaged deployments use Go APIs instead. |
| `web/lib/{redis,types,utils,webhook}.ts` | Local-dashboard Redis access, shared client types/helpers, and HMAC helper. |
| `web/{next.config.ts,postcss.config.mjs,package.json,tsconfig.json}` | Console build, Tailwind/PostCSS, dependency, and TypeScript configuration. |
| `website/app/page.tsx` | Public landing page. |
| `website/app/docs/*` | Fumadocs route layout and dynamic MDX page rendering. |
| `website/content/docs/*.mdx`, `meta.json` | Public documentation pages and sidebar order. |
| `website/lib/source.ts` | Creates the Fumadocs MDX content loader. |
| `website/lib/layout.shared.tsx` | Shared Fumadocs navigation/title configuration. |
| `website/components/{site-header,mdx}.tsx` | Public-site top navigation and Fumadocs MDX component registration. |
| `website/{next.config.ts,postcss.config.mjs,package.json,tsconfig.json}` | Independent website build/dependency configuration. |
| `sample/source-init.sql` | Creates example source tables/data and enables replica identity needed for richer update/delete events. |
| `sample/receiver/*` | Go webhook receiver that checks HMAC and prints event information. |
| `release/docker-compose/*` | Minimal customer distribution: service definition, blank safe environment template, offline README. |
| `Dockerfile` | Multi-stage packaged server build: builds console, embeds it in Go binary, and emits non-root runtime image. |
| `docker-compose.yml`, `Makefile` | Local development Redis and metadata helper workflow. |
| `scripts/package-release.ps1` | Validates tracked release files, creates a clean ZIP, and prints SHA-256 checksum. |
