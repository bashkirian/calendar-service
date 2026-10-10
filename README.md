# calendar-service

Go service: stores calendar entries in MySQL, publishes active ones to SQS (LocalStack); a consumer prints them.

## Run

Needs Docker. Go 1.26 only to run outside containers.

```sh
make app                               # MySQL, LocalStack, migrations, api (:8080), consumer
docker compose logs -f consumer        # in another terminal: printed entries

# or on the host
make up && make api                    # infra in Docker, api via go run
make consumer                          # in another terminal
make test                              # unit + integration (testcontainers)
```

```sh
curl -XPOST localhost:8080/v1/calendar-entries \
  -d '{"startDate":"2026-10-12T09:00:00+02:00","stopDate":"2026-10-12T11:30:00+02:00"}'
curl localhost:8080/v1/calendar-entries/active
curl -XPOST localhost:8080/v1/calendar-entries/active/publish
```

## Endpoints

| Route | Result |
|---|---|
| `POST /v1/calendar-entries` | 201 entry; 422 stop before start; 400 malformed; 413 over 1 MiB |
| `GET /v1/calendar-entries/active` | `{"items":[...]}`, entries with `stopDate >= now` |
| `POST /v1/calendar-entries/active/publish` | sends all active entries to SQS, `{"published":n}` |

Errors are RFC 9457 `application/problem+json`. `/healthz`, `/readyz` for probes.

## Code

```
cmd/api, cmd/consumer, cmd/migrate   wiring, scheduler loop, console printer
internal/calendar                    Entry, validation, use cases, ports
internal/mysql                       repository, migrations, schedule claim
internal/sqs                         publisher, consumer, message format
internal/httpapi                     handlers
```

## Test (task 1d)

The case: **an entry is active while `stopDate` is not in the past**
(`internal/mysql/repository_test.go`, `TestListActive`).
The rule lives in SQL, so the test runs against real MySQL 8.4 (testcontainers, production migrations)
and checks the boundary: `stop = now - 1ms` is excluded, `stop = now` is included. A mock would test nothing here.

Also covered: once-only marking, one scheduled run across replicas, SQS publish → consume with redelivery.
`go test -short ./...` skips the Docker tests.

## Queues

- **Scheduled (every 5 min).** Each api replica ticks on start and every `PUBLISH_INTERVAL`.
  Before a run it moves `next_run_at` in the `scheduled_jobs` table forward; only one replica can do that per interval.
- **Once only (best effort).** `PUBLISH_ONCE=true`: publish rows where `sent_at IS NULL`, then set `sent_at` for the IDs SQS accepted.
  A crash between the two resends them, so delivery is at-least-once and consumers dedupe by `data.id`.
  Stricter: transactional outbox, or a FIFO queue with `MessageDeduplicationId`.
- **Async consumer.** Long-polls, deletes only handled messages; failures retry and go to the DLQ after 5 attempts.

## Versioning

HTTP under `/v1`. Messages are `{type, version, occurredAt, data}`; unknown versions go to the DLQ. Schema changes via goose migrations.

## Left out on purpose

Pagination (keyset on the existing `(stop_date, id)` index when needed), auth on publish, metrics and tracing.

LocalStack is pinned to `4.14`: newer images need `LOCALSTACK_AUTH_TOKEN` (set `LOCALSTACK_TAG` to override).
