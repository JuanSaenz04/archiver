# Job logs

Each crawl attempt captures stdout/stderr through `io.MultiWriter`. Line writers
enqueue without waiting for Redis; a per-job publisher flushes every 200 ms or
100 entries. Container logging continues as before. Logging is best effort:
overflow and failed writes drop entries, with a gap message when publishing
recovers. Logs are captured even with no viewers.

Limits are defined in `joblogs.go`: 512 queued entries, 4 KiB of input per line,
2,000 retained stream entries, one-hour successful-job retention and 24-hour
failed-job retention. Writes refresh a 24-hour safety TTL for abandoned jobs.
Truncation markers and UTF-8 replacement can add a small amount to a line's size.
Worker log publishing uses a separate Redis client with short timeouts and no
retries. Closing a session allows two seconds to drain and then applies retention.
If Redis is unavailable during cleanup, the safety TTL remains the fallback.

`GET /api/jobs/:jobId/logs` serves SSE only for an open viewer. It sends the latest
500 entries, then reads forward every 250 ms, with up to 32 simultaneous viewers
per API process. Each viewer owns its cursor; there are no consumer groups or
background readers without viewers. `Last-Event-ID` resumes from the previous
batch. Events are `logs` (array of entries), `status` (string), `gap` (string), and
`done` (`status`, `unavailable`). Status events also keep idle connections alive.
Terminal jobs drain available logs before `done`. Closing the modal closes SSE;
request cancellation stops server-side reading. Slow writes have a five-second
deadline. Gzip is disabled for this route; reverse proxies must allow streaming
and an idle timeout longer than the 15-second heartbeat interval.

The modal renders plain text, batches updates and retains at most 2,000 lines.
It does not interpret HTML or terminal escape sequences. Log content receives
the same access controls as the jobs API. Raw crawler output can contain URLs or
other sensitive data; this package does not attempt generic secret detection.
Retention limits apply to Redis history, not existing container logs. This is
temporary diagnostic history, not durable audit storage.

Validation: `go test -race ./...`, and in `front/`, `pnpm test`, `pnpm lint`,
`pnpm build`. Frontend protocol tests use Node's built-in test runner (Node 22.18+
or 24+) and do not require a browser or additional test dependencies.
