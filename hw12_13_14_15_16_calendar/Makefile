BIN := "./bin/calendar"
SCHEDULER_BIN := "./bin/calendar_scheduler"
SENDER_BIN := "./bin/calendar_sender"
DOCKER_IMG="calendar:develop"

GIT_HASH := $(shell git log --format="%h" -n 1)
LDFLAGS := -X main.release="develop" -X main.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%S) -X main.gitHash=$(GIT_HASH)

#Postrges
POSTGRES_USER ?= postgres
POSTGRES_PASSWORD ?= password
POSTGRES_DB ?= backend
POSTGRES_PORT ?= 5435
POSTGRES_CONTAINER := postgres-calendar

#Rabbit
RABBIT_USER ?= rabbit
RABBIT_PASSWORD ?= password
RABBIT_PORT ?= 5672
RABBIT_PORT_UI ?= 15672
RABBIT_CONTAINER := rabbitmq


run-postgres:
	docker run -d --name $(POSTGRES_CONTAINER) \
	-e POSTGRES_USER=$(POSTGRES_USER) \
	-e POSTGRES_PASSWORD=$(POSTGRES_PASSWORD) \
	-e POSTGRES_DB=$(POSTGRES_DB) \
	-p $(POSTGRES_PORT):5432 \
	-v postgres-data:/var/lib/postgresql/data \
	postgres:latest

run-rabbit:
	docker run -d --name $(RABBIT_CONTAINER) \
	-p $(RABBIT_PORT):5672 \
	-p $(RABBIT_PORT_UI):15672 \
	-e RABBITMQ_DEFAULT_USER=$(RABBIT_USER) \
	-e RABBITMQ_DEFAULT_PASS=$(RABBIT_PASSWORD) \
	rabbitmq:management

build:
	go build -v -o $(BIN) -ldflags "$(LDFLAGS)" ./cmd/calendar
	go build -v -o $(SCHEDULER_BIN) ./cmd/calendar_scheduler
	go build -v -o $(SENDER_BIN) ./cmd/calendar_sender

run: build
	$(BIN) -config ./configs/config.yaml

build-img:
	docker build \
		--build-arg=LDFLAGS="$(LDFLAGS)" \
		-t $(DOCKER_IMG) \
		-f build/Dockerfile .

run-img: build-img
	docker run $(DOCKER_IMG)

version: build
	$(BIN) version

test:
	go test -count=1 -race ./...

install-lint-deps:
	(which golangci-lint > /dev/null) || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/main/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v2.13.2

lint: install-lint-deps
	golangci-lint run ./...

.PHONY: build run build-img run-img version test lint install-lint-deps run-postgres run-rabbit migrate test-sql

# Apply the schema explicitly before starting with storage.type=sql.
migrate:
	@test -n "$(DATABASE_DSN)" || (echo "Set DATABASE_DSN to the target PostgreSQL database"; exit 1)
	psql "$(DATABASE_DSN)" -v ON_ERROR_STOP=1 -f migrations/001_events.up.sql
	psql "$(DATABASE_DSN)" -v ON_ERROR_STOP=1 -f migrations/002_notifications.up.sql

test-sql:
	@test -n "$(CALENDAR_TEST_DSN)" || (echo "Set CALENDAR_TEST_DSN to a dedicated PostgreSQL test database"; exit 1)
	go test -count=1 -race -v ./internal/storage/sql

# protoc is supplied separately; generated files are stored in internal/pb.
install-generate-deps:
	GOBIN="$(CURDIR)/bin" go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.5
	GOBIN="$(CURDIR)/bin" go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

generate:
	go generate ./api

.PHONY: generate install-generate-deps

run-scheduler: build
	$(SCHEDULER_BIN) --config=./configs/scheduler_config.yaml

run-sender: build
	$(SENDER_BIN) --config=./configs/sender_config.yaml

.PHONY: run-scheduler run-sender
