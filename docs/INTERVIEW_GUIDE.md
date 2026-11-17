# How to explain CDC Lite in an interview

## 30-second version

“CDC Lite is a self-hosted PostgreSQL change-data-capture project. It reads committed database changes through PostgreSQL logical replication and the `pgoutput` protocol, buffers them in Redis Streams, and delivers signed webhook events. I built the Go replication and delivery backend, a Next.js operations console, SQLite-backed encrypted configuration, Docker packaging, and a separate Fumadocs documentation site.”

## 2-minute walkthrough

“A user saves a PostgreSQL connection in the console. I test regular SQL connectivity separately from logical replication so a normal connection does not incorrectly imply CDC support. After they choose tables, the server creates a publication and replication slot, then uses pgx/pglogrepl to decode the `pgoutput` messages.

I buffer changes after `Begin` and only publish them after the PostgreSQL `Commit` message. On commit, I make an event ID from the commit LSN plus sequence number and write events to Redis Streams before acknowledging the LSN to PostgreSQL. That is the core reliability decision: if Redis is unavailable, I do not acknowledge the LSN, so PostgreSQL can replay changes. The trade-off is at-least-once delivery, so receivers must deduplicate by event ID.

The delivery worker reads Redis through a consumer group. It sends an HMAC-SHA256 signature over the exact request body with a timestamp, records safe delivery history, and acknowledges Redis only after a 2xx webhook response. The dashboard displays live events through SSE and derives sink health from real delivery outcomes.”

## Design decisions worth highlighting

| Decision | Why it matters |
| --- | --- |
| Logical replication instead of polling | Captures committed database changes with lower latency and without repeatedly scanning tables. |
| Commit-time buffering | Prevents events from rolled-back or incomplete transactions from being emitted. |
| Redis before PostgreSQL acknowledgement | Prefers duplicates over data loss. |
| Stable event IDs | Lets consumers implement idempotency. |
| Redis Streams + consumer group | Separates capture speed from webhook speed and provides pending-message state. |
| HMAC over raw body | Lets the webhook verify message integrity and origin. |
| SQLite metadata | Minimal persistent control-plane store for one self-hosted installation. |
| Separate public website | Keeps public docs/landing concerns independent from the customer operations console. |

## Honest limitations to say proactively

- One running pipeline per installation.
- Sequential webhook delivery, with no advanced retry scheduler or DLQ.
- No initial snapshots or schema-evolution support.
- No login enforcement yet, despite administrator metadata existing.
- Current server is optimized for a working student demo and clear code, not production-scale multi-tenancy.

Saying these limitations clearly makes the design discussion stronger: explain what you intentionally did not build, then describe the next scaling steps.

## Demo script

1. Open the customer console and show a saved source database.
2. Browse `orders` and show table metadata / replica identity.
3. Show the running pipeline and enabled Webhook.site sink.
4. Insert an `orders` row in Neon SQL Editor.
5. Show the event appear in Live Events.
6. Show the signed event arrive in Webhook.site.
7. Open Sinks and show the event ID, successful delivery, and Healthy status.
8. Explain how retry can repeat an event and why a receiver deduplicates it.
