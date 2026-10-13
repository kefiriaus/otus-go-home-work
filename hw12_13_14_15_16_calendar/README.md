#### Результатом выполнения следующих домашних заданий является сервис «Календарь»:
- [Домашнее задание №12 «Заготовка сервиса Календарь»](./docs/12_README.md)
- [Домашнее задание №13 «Внешние API от Календаря»](./docs/13_README.md)
- [Домашнее задание №14 «Кроликизация Календаря»](./docs/14_README.md)
- [Домашнее задание №15 «Докеризация и интеграционное тестирование Календаря»](./docs/15_README.md)

#### Ветки при выполнении
- `hw12_calendar` (от `master`) -> Merge Request в `master`
- `hw13_calendar` (от `hw12_calendar`) -> Merge Request в `hw12_calendar` (если уже вмержена, то в `master`)
- `hw14_calendar` (от `hw13_calendar`) -> Merge Request в `hw13_calendar` (если уже вмержена, то в `master`)
- `hw15_calendar` (от `hw14_calendar`) -> Merge Request в `hw14_calendar` (если уже вмержена, то в `master`)
- `hw16_calendar` (от `hw15_calendar`) -> Merge Request в `hw15_calendar` (если уже вмержена, то в `master`)


**Домашнее задание не принимается, если не принято ДЗ, предшествующее ему.**

## Homework 12: running the skeleton

Run commands from `hw12_13_14_15_16_calendar`:

```sh
make build
make run
curl 'http://localhost:8080/hello?q=1'
make test
make lint
```

`make test` runs all unit tests with the race detector. PostgreSQL integration tests
skip unless `CALENDAR_TEST_DSN` is set. The hello route returns `Hello, calendar!`;
unknown paths return 404 and unsupported methods return 405. SIGINT/SIGTERM/SIGHUP
stop the server gracefully. `./bin/calendar version` prints build information.

### Configuration and logging

Pass a YAML file with `./bin/calendar --config=./configs/config.yaml`. The sample
uses memory storage, HTTP port 8080, gRPC port 50051, info logging to stdout and `calendar.log`. Set
`logger.file` to an empty string for stdout only. The optional log file's parent
directory must already exist. Logger levels are `error`, `warn`, `info`, `debug`.
Request records include client IP, timestamp, method, URI including query,
HTTP protocol, status, latency and user agent. Request records use the info level;
choose info/debug to retain them.

These environment variables override YAML values:

| Variable | Field |
| --- | --- |
| `CALENDAR_LOG_LEVEL` | `logger.level` |
| `CALENDAR_LOG_FILE` | `logger.file` |
| `CALENDAR_HTTP_HOST` | `http.host` |
| `CALENDAR_HTTP_PORT` | `http.port` |
| `CALENDAR_GRPC_HOST` | `grpc.host` |
| `CALENDAR_GRPC_PORT` | `grpc.port` |
| `CALENDAR_STORAGE_TYPE` | `storage.type` (`memory` or `sql`) |
| `CALENDAR_DATABASE_DSN` | `database.dsn` |

Configuration rejects unknown fields, malformed YAML, invalid levels/ports and
SQL storage without a DSN. Missing fields receive defaults (info, stdout,
HTTP 0.0.0.0:8080, gRPC 0.0.0.0:50051, memory). Supply a config file even when using environment overrides.

### PostgreSQL

Use a dedicated database and apply the migration before starting SQL storage:

```sh
make run-postgres
make migrate DATABASE_DSN='postgres://postgres:password@localhost:5435/backend?sslmode=disable'
CALENDAR_STORAGE_TYPE=sql \
CALENDAR_DATABASE_DSN='postgres://postgres:password@localhost:5435/backend?sslmode=disable' \
make run
```

The migration requires permission to install PostgreSQL's `btree_gist` extension.
Credentials and the database name for the development container are configurable
through the Makefile's `POSTGRES_*` variables. SQL queries use `database/sql` and
`lib/pq`, without an ORM. The GiST exclusion constraint prevents simultaneous
overlapping inserts/updates for the same user. The start-time index supports
calendar listings. `001_events.down.sql` drops the events table; run it only when
its data is no longer needed. Migrations are manual and are not applied by startup.

To test SQL behavior, point at a **dedicated test database**. Tests apply the up
migration and remove their own generated events afterward:

```sh
CALENDAR_TEST_DSN='postgres://postgres:password@localhost:5435/calendar_test?sslmode=disable' make test-sql
```

Create `calendar_test` beforehand. Do not run these tests against a production
or shared database.

### Event and storage semantics

Events contain a caller-supplied ID, title, start time, positive duration,
optional description, user ID and optional nonnegative reminder duration.
Both storage implementations return `ErrAlreadyExists`, `ErrNotFound`,
`ErrDateBusy`, `ErrInvalidEvent` and `ErrInvalidRange` for the corresponding
business errors. Events owned by different users may overlap; intervals are
half-open, so adjacent events are allowed. A rejected update preserves old data.

Event start times/durations use microsecond precision, matching PostgreSQL;
timestamps must be within years 1–9999. In-memory storage copies optional
reminder pointers on input and output and protects all access with an RWMutex.
Listings select events **starting** in `[from, to)` and sort by start time, then
ID. Query bounds may use nanosecond precision; SQL rounds bounds upward to
microseconds so both backends preserve these comparisons. Day/week/month operations use calendar boundaries in the supplied timezone,
including daylight saving transitions. Week starts on the supplied date; month
starts on the first day of that date's month.

At the homework 12 stage only the hello-world route was exposed. The current
HTTP/gRPC API is described below.


## Homework 13: HTTP and gRPC API

Both servers start with `make run`, share one application/storage and shut down
together. Graceful shutdown allows up to three seconds; active RPCs that ignore
cancellation cannot indefinitely block process exit. Timezone data is embedded
for IANA support in minimal container images. A failure to bind either port stops
the other server. The application
and storage packages have no dependencies on HTTP, Protobuf or gRPC.

### HTTP contract

All event requests/responses use JSON with camelCase field names. Event IDs are
supplied by the caller. `startsAt` is an RFC3339 timestamp; `duration` and optional
`notifyBefore` are Go duration strings (for example `1h`, `30m`, `0s`). Omit
`notifyBefore` for no reminder; `0s` is an explicit reminder at the start.
Timestamps are normalized to UTC, including validation of the Protobuf year range.
The existing microsecond precision and per-user overlap rules still apply.

| Method and path | Request | Success |
| --- | --- | --- |
| `POST /events` | Event JSON | 201, `{"event": {...}}` |
| `PUT /events/{id}` | Full Event JSON, ID must match path | 200, `{"event": {...}}` |
| `DELETE /events/{id}` | No body | 204 |
| `GET /events/day` | `date`, optional `timezone` query parameters | 200, `{"events": [...]}` |
| `GET /events/week` | Same | 200, `{"events": [...]}` |
| `GET /events/month` | Same | 200, `{"events": [...]}` |

`date` uses `YYYY-MM-DD`. `timezone` is an IANA name, default UTC. Week starts on
that date; month starts on its first day. Daylight saving transitions follow the
chosen timezone. Empty lists return an empty JSON array. `/hello` remains available.

```sh
curl -i -X POST http://localhost:8080/events \
  -H 'Content-Type: application/json' \
  -d '{"id":"meeting-1","title":"Meeting","startsAt":"2026-10-13T10:00:00+03:00","duration":"1h","userId":"alice","notifyBefore":"30m"}'
curl 'http://localhost:8080/events/day?date=2026-10-13&timezone=Europe%2FMoscow'
curl -i -X DELETE http://localhost:8080/events/meeting-1
```

Error bodies contain `{"error":"..."}`. Invalid JSON/fields/dates/events return
400, missing events 404, duplicates or overlaps 409, oversized bodies 413,
and unsupported Content-Type 415. Unexpected backend errors return a generic
500 without exposing database details. Bodies are limited to 1 MiB.

### gRPC contract and generation

`api/EventService.proto` defines `calendar.v1.EventService` with all six methods,
request/response messages and a reusable Event. Event timestamps/durations use
Protobuf's `Timestamp`/`Duration`. List requests contain `date` and `timezone`
with the same semantics as HTTP. The server validates Protobuf values before
conversion, including overflow beyond Go's duration range.

Business errors map to InvalidArgument, NotFound and AlreadyExists; context
errors map to Canceled/DeadlineExceeded and unexpected errors to Internal.
Every supported unary request logs peer IP, time, full method name, gRPC status,
latency and user agent.

Generated client/server bindings are stored in `internal/pb` and included in the
repository, so normal builds/tests do not require generation tools. To regenerate:

1. Install the official `protoc` compiler (verified with 29.3).
2. Run `make install-generate-deps` to install pinned Go plugins into `bin`.
3. Run `make generate`, which calls `go generate ./api`.

Set `PROTOC=/path/to/protoc` to use a compiler outside PATH. Plugin versions are
`protoc-gen-go` 1.36.5 and `protoc-gen-go-grpc` 1.5.1. The generated artifacts are
not manually formatted or edited.

Example with grpcurl (uses the supplied schema, reflection is unnecessary):

```sh
grpcurl -plaintext -import-path api -proto EventService.proto \
  -d '{"date":"2026-10-13","timezone":"Europe/Moscow"}' \
  localhost:50051 calendar.v1.EventService/ListEventsOnDay
```

`make test` includes real gRPC client/server tests using bufconn and HTTP handler
tests against the memory-backed application. Real PostgreSQL tests remain opt-in
through `CALENDAR_TEST_DSN`.
