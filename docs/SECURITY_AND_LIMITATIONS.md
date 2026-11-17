# Security, trade-offs, and limitations

## Security controls implemented

- PostgreSQL URLs and webhook signing secrets are encrypted at rest with a persistent `APP_ENCRYPTION_KEY`.
- Administrator passwords are stored as bcrypt hashes, never plaintext.
- Database credentials, full URLs, and webhook secrets are not logged or returned from APIs.
- Errors are redacted before they enter stored delivery history.
- Remote PostgreSQL connections require a secure SSL mode.
- Database browsing uses read-only transactions, timeouts, identifier quoting, and limited previews; arbitrary browser SQL is not accepted.
- Webhook requests are signed with HMAC-SHA256 over exact raw bodies.
- Webhook target validation blocks private/loopback/link-local/metadata ranges, rechecks DNS at connection time, and refuses redirects.
- Docker deployment runs as a non-root user and keeps Redis off the host network by default.

## Intentional limitations

| Limitation | Why it exists in this version |
| --- | --- |
| One active pipeline | Keeps runtime coordination understandable. |
| One sink type: Webhook | Demonstrates the event delivery boundary without building every integration. |
| Sequential delivery | Avoids ordering/concurrency complexity for a student project. |
| No advanced retry scheduler or DLQ | Basic pending-message behavior demonstrates reliability without building a full delivery platform. |
| No snapshot/schema evolution | Focus remains on committed ongoing changes for known tables. |
| No enforced login UI | Administrator metadata is in place; full authentication is intentionally future scope. |
| SQLite metadata | Good for a single installation; unsuitable for horizontally scaled control plane writes. |

## How to present the trade-offs

“I optimized the first version for a correct and demonstrable data path rather than feature count. The important reliability path is transaction buffering, Redis-before-LSN acknowledgement, stable event IDs, and safe delivery status. I documented the gaps that would need distributed coordination or additional infrastructure, instead of hiding them behind a production claim.”
