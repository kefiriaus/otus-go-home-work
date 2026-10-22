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
make run-postgres
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
uses SQL storage (shared with the scheduler), HTTP port 8080, gRPC port 50051, info logging to stdout and `calendar.log`. For an isolated API-only demo, set `storage.type` to `memory`. Set
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
its data is no longer needed. API SQL startup and scheduler startup apply embedded up migrations automatically.
`make migrate` remains available for explicit schema setup.

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


## Homework 14: reminders via RabbitMQ

`make build` produces `bin/calendar`, `bin/calendar_scheduler`, and
`bin/calendar_sender`. All accept `--config` with a YAML path. Start PostgreSQL
and RabbitMQ, then run the workers in separate terminals:

```sh
make run-postgres
make run-rabbit
./bin/calendar_scheduler --config=configs/scheduler_config.yaml
./bin/calendar_sender --config=configs/sender_config.yaml
./bin/calendar --config=configs/config.yaml
```

Sample configs agree on the database and queue. API sample storage is now SQL;
using memory storage for the API does not share events with the scheduler.
No manual queue/exchange/schema creation is required: scheduler/API/sender apply schema
migrations and both workers declare the durable notification queue, its dead-letter
exchange and `<queue>.rejected` queue on startup. Embedded migrations use a database
advisory lock so API and both workers can start concurrently. Existing data is preserved.

### Scheduling and delivery

Scheduler scans immediately and then every configured interval. It selects events
with a non-null reminder, no sent marker, reminder time at/before now and end time
after now. `notifyBefore: "0s"` becomes due when the event starts. A missed reminder
can still be delivered while the event is ongoing; finished events are not sent.
Each batch locks rows with `FOR UPDATE SKIP LOCKED`, publishes notifications and
marks them sent only after RabbitMQ publisher confirmations. Concurrent API edits
wait for locked rows; changing start time or reminder interval resets the marker.
Description/title-only updates do not resend an already published reminder.

Notifications are JSON (`eventId`, `title`, `startsAt`, `userId`), not stored as
separate database records. Queue/messages are durable/persistent. Mandatory routing
and publisher confirms detect rejected/unroutable publications. Delivery is
**at-least-once**: a crash or transaction failure after publication can produce
repeats. The marker prevents routine repeated scans/restarts from republishing.

Cleanup removes events whose end is strictly earlier than `now.AddDate(-1,0,0)`;
an event exactly on the cutoff remains. Cleanup runs on each scan.
Sender writes each valid notification as JSON to stdout (and the optional file),
independent of diagnostic log-level filtering, records a SQL delivery receipt, then ACKs. Output or receipt failure requeues
the message and stops the sender. Invalid JSON/schema is
NACKed without requeue and goes to the rejected queue. Handler failures requeue the
message and stop the process. Broker/database failures exit visibly; use a process
supervisor to restart workers. Operations have timeouts and signals stop workers.

### Worker settings

Both workers share `internal/config` and accept the existing logger settings.
Sample worker logging is stdout-only. The optional logger file works as for API.

| YAML field | Environment override | Default |
| --- | --- | --- |
| `database.dsn` | `CALENDAR_DATABASE_DSN` | Required for SQL API and both workers |
| `rabbitmq.url` | `CALENDAR_RABBITMQ_URL` | `amqp://localhost:5672/` (broker default credentials) |
| `rabbitmq.queue` | `CALENDAR_RABBITMQ_QUEUE` | `calendar.notifications` |
| `rabbitmq.prefetch` | `CALENDAR_RABBITMQ_PREFETCH` | 10 |
| `rabbitmq.timeout` | `CALENDAR_RABBITMQ_TIMEOUT` | `5s` |
| `scheduler.interval` | `CALENDAR_SCHEDULER_INTERVAL` | `1m` |
| `scheduler.batchSize` | `CALENDAR_SCHEDULER_BATCH_SIZE` | 100 |
| `scheduler.timeout` | `CALENDAR_SCHEDULER_TIMEOUT` | `10s` per scan |

Explicit worker sample URLs use the same `rabbit/password` credentials as
`make run-rabbit`. Timeout/interval/batch/prefetch values must be positive. RabbitMQ timeout also
bounds waiting for the consumer-registration response.

Unit tests run with `make test`. To include real PostgreSQL and RabbitMQ tests,
use dedicated test resources:

```sh
CALENDAR_TEST_DSN='postgres://postgres:password@localhost:5435/calendar_test?sslmode=disable' \
CALENDAR_TEST_RABBITMQ_URL='amqp://rabbit:password@localhost:5672/' make test
```

The SQL tests verify due selection, concurrent dispatch, failure retry, marker reset
and cleanup boundaries. RabbitMQ integration tests use unique queue names; dispose
of the dedicated broker or remove `calendar.test.*` queues/exchanges afterwards.


## Homework 15: Docker and end-to-end tests

Requires Docker Engine/Desktop and Docker Compose v2.35 or newer. Run from this
calendar directory:

```sh
make up
curl http://localhost:8888/hello
make down
```

`make up` builds three independent application images and starts PostgreSQL 17.6,
RabbitMQ 4.1.4, Calendar API, Scheduler and Sender. PostgreSQL and RabbitMQ
healthchecks gate application startup. All processes apply embedded migrations
safely at startup; no manual migration step is required. HTTP is published on
`127.0.0.1:8888`, gRPC on `127.0.0.1:50051`. `docker compose up --build` also works
from this directory. Database/broker ports are internal to the Compose network.
Credentials in Compose are local development examples.

All application images run as an unprivileged user and write logs to stdout;
`docker compose logs -f api scheduler sender` follows them. Configuration files are
included in images; existing `CALENDAR_*` environment variables override their
settings, including internal database/broker addresses and an empty log-file path.
The shared service definitions are in `deployments/docker-compose.yaml`; root
`compose.yaml` adds host ports and a persistent PostgreSQL volume. `make down`
preserves that volume. To explicitly discard local calendar data, use
`docker compose down --volumes`.

### Delivery receipts

Sender now requires PostgreSQL (`database.dsn` / `CALENDAR_DATABASE_DSN`), just like
Scheduler. After logging a notification, it writes a row to
`notification_deliveries(event_id, starts_at, delivered_at)` before ACK. The key
`(event_id, starts_at)` makes receipt writes idempotent and permits a new receipt
when an event moves to a new start time. Receipts store status, not notification
bodies, and remain after event deletion. Receipt writes are bounded by `rabbitmq.timeout`. There is no automatic receipt retention
policy. Delivery remains at-least-once: a failure after stdout but before receipt
or ACK can repeat the log entry. A receipt proves the sender processed the message;
the scheduler's publication marker alone does not prove delivery.

### Isolated integration suite

```sh
make integration-tests
```

This builds and starts a separate Compose project with all five services and a Go
test container, runs the `integration` package, then removes its containers,
network and volumes. No host ports are bound, and the ordinary calendar environment
and its data are untouched. Concurrent runs use different project names. Success
returns 0; build, startup, test or cleanup failure returns 1 from the runner
(`make` itself conventionally reports failed recipes with status 2). Failure logs
are printed before cleanup. Interruptions also trigger cleanup.

The suite tests creation, validation/duplicate/time-conflict/not-found errors,
update/delete, exact day/week/month listing boundaries, and actual notification
delivery through the sender's SQL receipt. It verifies future reminders are not
delivered prematurely. Readiness and delivery waits have deadlines.

The suite uses the `integration` build tag and is separate from `make test`.
For a deliberate failure/cleanup check:

```sh
CALENDAR_INTEGRATION_API_URL=http://api:1 make integration-tests
```


## Homework 16: Kubernetes and Helm

The self-contained chart lives in `deployments/calendar-chart`. It deploys three
application Deployments, a Service for each, an HTTP Ingress, PostgreSQL/RabbitMQ
StatefulSets, Secrets, a ConfigMap and persistent volumes. Default application
images are local `calendar-{api,scheduler,sender}:hw16` images; they do not require
a public registry. Infrastructure uses the same official images as Compose.

### Local cluster

Install Docker, Minikube, Helm and kubectl from their official distributions.
The tested tool versions and real deployment results are recorded in
`docs/16_KUBERNETES_RESULTS.md`. The default Kubernetes server version is v1.35.0;
use a kubectl client within the supported one-minor version skew.

```sh
make k8s-start       # dedicated calendar-hw16 Minikube Docker profile, 2 CPU / 4 GiB
make k8s-images      # builds and loads the 3 application images + test image
make helm-check     # Helm/schema/render contracts, generated YAML consistency
make k8s-deploy      # helm upgrade --install, wait for healthy resources
make k8s-status
make k8s-smoke       # existing business integration suite, now in a Kubernetes Job
```

The scripts use `.local/hw16/kubeconfig` and `.local/hw16/minikube` rather than the
user's default kubeconfig/context. Every cluster operation explicitly selects the
local `calendar-hw16` context; an unrelated context is rejected before any request.
The namespace is `calendar-hw16`, the release is `calendar`. Existing remote
clusters are not used. `.local/` is ignored by Git. To use task-local executables
or a different state directory, set `MINIKUBE`, `HELM`, `KUBECTL` and
`CALENDAR_K8S_STATE`. `PROFILE` may be `calendar-hw16` or start with
`calendar-hw16-`; `KUBE_CONTEXT` must equal that profile. `NAMESPACE` and `RELEASE`
can be overridden. Install internet access is needed for initial cluster/images.

### Access through Ingress

```sh
make k8s-ingress
# In another terminal:
curl -H 'Host: calendar.local' http://127.0.0.1:18888/hello
```

This forwards the actual ingress controller, so requests pass through the Ingress
rule. It works with Docker-backed Minikube on macOS without modifying `/etc/hosts`
or reserving privileged host ports. Stop the foreground port-forward with Ctrl-C.
For direct API access instead, `make k8s-api` forwards HTTP 18888 and gRPC 15051.
Worker Services are headless discovery Services: Scheduler/Sender have no inbound
HTTP protocol or fabricated listening ports. Only the API is routed by Ingress.
API probes use `/hello`, which checks process health, not live DB connectivity.

### Helm configuration and standalone YAML

`values.yaml` contains development defaults for images/pull policies, replicas,
resources, API ports, Ingress host/class/annotations, scheduler timings, queue,
connection settings and infrastructure persistence. Schema validation rejects
invalid values; HTTP/gRPC ports must differ. Bundled database/broker credentials
are deliberately local examples and restricted to URL-safe characters. Use an
existing Secret for real credentials; never put them in a committed values file.

For a custom Helm invocation, select the dedicated kubeconfig/context explicitly:

```sh
helm upgrade --install calendar deployments/calendar-chart \
  --kubeconfig .local/hw16/kubeconfig --kube-context calendar-hw16 \
  --namespace calendar-hw16 --create-namespace --wait --timeout 10m \
  --set ingress.host=my-calendar.local
```

To use external infrastructure, set `postgres.enabled=false` and/or
`rabbitmq.enabled=false`, and supply `connections.databaseDSN` /
`connections.rabbitmqURL`, or `connections.existingSecret`. The existing Secret
must be in the release namespace with keys `database-dsn` and `rabbitmq-url`.
With bundled infrastructure enabled, credentials in that Secret must match the
bundled service credentials. Config/managed Secret changes restart application
Pods; when using an external Secret, restart the Deployments after changing its
values. Do not rotate database/broker credentials on existing PVCs by just editing
Helm values: the existing data retains its original credentials.

Plain manifests are generated from the same chart to avoid two independent sets
of configuration. `make k8s-render` regenerates
`deployments/kubernetes/calendar.yaml`; `make helm-check` rejects drift.
As an alternative to Helm installation, use a fresh namespace/cluster:

```sh
kubectl --kubeconfig .local/hw16/kubeconfig --context calendar-hw16 \
  create namespace calendar-hw16
kubectl --kubeconfig .local/hw16/kubeconfig --context calendar-hw16 \
  apply -f deployments/kubernetes/calendar.yaml
```

Do not combine standalone apply and Helm ownership for the same resources. The
committed YAML uses release `calendar` and namespace `calendar-hw16`; render with
explicit Helm arguments for another name/namespace. Migrations run automatically
on application startup under the existing advisory lock. PostgreSQL startup waits
are bounded; transient RabbitMQ startup failures cause Kubernetes to restart the
worker after its connection timeout.

### Tests, evidence and persistence

`make k8s-smoke` reads the installed Helm release values (including custom ports,
images and Secret names) and explicitly creates a finite Job using the existing integration
image, waits for success or failure, prints test logs and returns nonzero on
failure. The Job covers CRUD/business errors, exact day/week/month boundaries and
real sender receipts. It is not run automatically by Helm install. Use
`SMOKE_API_URL=http://calendar-api:1 make k8s-smoke` for a controlled failure with
default release names. The smoke helper requires a Helm-managed release; for standalone apply, render the
optional test Job with the same values and apply it explicitly. The Job remains
for inspection and is replaced on the next
smoke run. It does not have access to the Kubernetes API token.

`make k8s-undeploy` uninstalls the release; StatefulSet PVCs remain. `make k8s-stop`
stops Minikube while retaining the cluster and data. To restart, run
`make k8s-start` and inspect `make k8s-status`. Data persists through cluster stop
and Helm uninstall/reinstall with the same release and namespace.

Deleting the namespace/PVCs or the Minikube profile discards homework data. These
are separate manual actions, not part of normal stop/uninstall. For explicit full
profile removal, use the matching local state and profile:

```sh
MINIKUBE_HOME="$PWD/.local/hw16/minikube" \
KUBECONFIG="$PWD/.local/hw16/kubeconfig" minikube -p calendar-hw16 delete
```

See `docs/16_KUBERNETES_RESULTS.md` for actual results and screenshots under
`docs/images/hw16/`, ready to attach to the homework MR. The deployment scripts do
not publish anything to GitHub.
