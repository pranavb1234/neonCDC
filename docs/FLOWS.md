# Walflow application flows

## How to read this document

Each flow names the current browser component, Go handler, store method, or package function. The packaged server is `worker/cmd/server`; the older standalone `cmd/connector` and `cmd/delivery` programs are documented in the component reference.

## 1. First startup and metadata initialization

1. Docker Compose starts `redis` and `walflow-server`.
2. The server opens `/data/walflow.db` in the persistent `walflow-data` volume.
3. SQLite migrations create administrator, source connection, pipeline, sink, and delivery-attempt tables.
4. `APP_ENCRYPTION_KEY` is loaded and used only in memory to encrypt/decrypt connection strings and webhook secrets.
5. On the first run, `ADMIN_EMAIL` and `ADMIN_PASSWORD` create one administrator record; only the bcrypt hash is stored.
6. The server checks Redis and exposes `/healthz` for Docker health checks.

## 2. Save and test a PostgreSQL connection

1. The browser submits a display name and PostgreSQL connection string to the server.
2. The server parses it with pgx, applies a connection timeout, and rejects insecure remote connections without a secure SSL mode.
3. It tests a normal SQL connection first.
4. It then checks logical replication prerequisites: `wal_level=logical` and whether the current role has `REPLICATION`.
5. The browser receives a safe success/error result; the full connection string is never returned.
6. On save, the connection string is encrypted and stored in SQLite.

## 3. Browse schemas, tables, and previews

1. The browser requests schemas for a saved connection.
2. The server decrypts the URL only in memory and opens a short-lived read-only PostgreSQL transaction.
3. It excludes system schemas and returns allowed schema names.
4. For a selected table it queries catalog metadata for columns, primary key, and replica identity.
5. A preview uses quoted schema/table identifiers and a strict small row limit. No browser-provided SQL is executed.

## 4. Create and start a CDC pipeline

1. The customer selects tables and names the pipeline.
2. The server revalidates that the chosen tables exist and logical replication is ready.
3. It creates stable internal publication and slot names within PostgreSQL identifier limits.
4. The pipeline configuration is persisted before the runtime job begins.
5. The server starts one Go replication job for the pipeline. The first release rejects an attempt to start a second pipeline.
6. The job creates or reuses the publication/slot, starts `pgoutput`, and maintains a relation-ID to column-metadata cache.

## 5. Decode and publish a committed source change

1. PostgreSQL sends `Begin`, relation metadata, row messages, and `Commit` through logical replication.
2. Walflow decodes relation, insert, update, and delete messages. It supports the common types used by sample tables: text, integers, booleans, numeric values, timestamps, and SQL `NULL`.
3. Row changes are held in a transaction buffer; nothing is emitted during an uncommitted transaction.
4. On `Commit`, each event receives a stable ID: `commit_lsn:sequence`.
5. The batch is written to Redis Stream `cdc:events` in transaction order, with the pipeline ID and JSON payload.
6. Only after Redis succeeds does Walflow send PostgreSQL standby status and acknowledge that LSN.

This ordering is important: a Redis outage causes a replay rather than silently losing the source transaction.

## 6. Deliver a webhook event

1. The delivery loop reads `cdc:events` using the `server-delivery` Redis consumer group.
2. It uses the stream's `pipeline_id` to load only enabled sinks attached to that same pipeline.
3. The sink URL is validated against SSRF rules before it is used and again at dial time after DNS resolution.
4. Walflow generates a timestamp and calculates HMAC-SHA256 over `timestamp + "." + raw JSON body`.
5. It sends `X-CDC-Event-Id`, `X-CDC-Timestamp`, and `X-CDC-Signature` headers.
6. A 2xx response is recorded as success and the Redis message is acknowledged. Failures are recorded safely and left pending for later retry.

The behavior is at-least-once: a message can be delivered more than once, especially after retry/recovery. Receivers should deduplicate with the event ID.

## 7. Live events and sink health

1. The Live Events page reads recent Redis entries, then opens an SSE connection.
2. The server waits for new stream entries and sends event payloads and their Redis stream IDs to the browser.
3. The Sinks page loads recent delivery attempts from SQLite.
4. Health is calculated from delivery evidence, not configuration: Healthy, Degraded, Failing, Paused, or Unknown.
5. Only safe metadata is retained: event ID, time, outcome, HTTP status, and redacted error. The database keeps the newest 100 attempts per sink and shows 20.

## 8. Exact end-to-end call chain for one `orders` INSERT

Assume a running pipeline selected `public.orders` and has an enabled webhook sink.

```text
1. User executes INSERT ...; COMMIT; in PostgreSQL.
2. PostgreSQL writes WAL and sends pgoutput frames through the replication slot.
3. replication.readMessages() receives pgproto3.CopyData.
4. pglogrepl.ParseXLogData() extracts WALData.
5. pglogrepl.Parse() returns BeginMessage, InsertMessage, then CommitMessage.
6. eventDecoder.Process(BeginMessage) starts a transaction buffer.
7. eventDecoder.Process(InsertMessage) calls insert() → relation() → decodeTuple() → decodeValue().
8. The decoded row is stored in eventDecoder.changes. Nothing is published yet.
9. eventDecoder.Process(CommitMessage) calls commit(), creating Event{id: commitLSN:1, ...}.
10. readMessages() calls StreamPublisher.Publish(ctx, events).
11. Publish() JSON-marshals the event, adds pipeline_id, queues Redis XADD, then Exec() persists it.
12. readMessages() calls acknowledgeCommit() only after Publish() returns nil.
13. deliveryLoop() reads the stream message with XREADGROUP.
14. It calls Store.EnabledSinksForPipeline(pipelineID), so unrelated sinks are excluded.
15. deliverWebhook() validates URL, decrypts secret in memory, signs raw JSON, and sends POST.
16. Store.RecordDeliveryAttempt() records the safe result. On 2xx, deliveryLoop() calls XACK.
17. The SSE endpoint observes the same Redis entry and EventList renders it in Live Events.
18. GET /api/sinks calls ListSinks() and sinkHealth(), so the delivery becomes visible in the overview.
```

## 9. What happens during common failures

### Redis fails at commit time

`StreamPublisher.Publish` fails. `readMessages` returns before `acknowledgeCommit`, so PostgreSQL has not been told the LSN is safe. A reconnect can replay the committed change. This protects against loss but can duplicate delivery.

### Webhook returns HTTP 500

`deliverWebhook` returns `webhookDeliveryError`. `deliveryLoop` records a failed `DeliveryAttempt`, does **not** `XACK`, and leaves the message pending. The sink health becomes Degraded or Failing based on recent attempt history.

### Server restarts with a pipeline recorded as running

The live cancellation map is process memory. At startup `MarkInterruptedPipelinesStopped` marks persisted `running`/`starting` pipelines stopped with a safe explanation. The saved source tables, slot/publication names, and sink configuration remain available for explicit restart.

### Browser closes Live Events

`stream()` observes `r.Context().Done()` and stops waiting on Redis. SSE does not consume or acknowledge stream messages; it only observes them, so the delivery pipeline continues independently.
