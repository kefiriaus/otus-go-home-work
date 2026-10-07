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
uses memory storage, port 8080, info logging to stdout and `calendar.log`. Set
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
| `CALENDAR_STORAGE_TYPE` | `storage.type` (`memory` or `sql`) |
| `CALENDAR_DATABASE_DSN` | `database.dsn` |

Configuration rejects unknown fields, malformed YAML, invalid levels/ports and
SQL storage without a DSN. Missing fields receive defaults (info, stdout,
0.0.0.0:8080, memory). Supply a config file even when using environment overrides.

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

Homework 12 initializes the application and its storage but exposes only the
hello-world HTTP route. Business HTTP/gRPC APIs belong to homework 13.
