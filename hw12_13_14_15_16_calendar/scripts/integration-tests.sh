#!/bin/sh
# Isolate every run, including concurrent runs and the regular calendar stack.
set -u
cd "$(dirname "$0")/.." || exit 1
project="calendar-it-$(date +%s)-$$"
compose() {
    docker compose -p "$project" -f deployments/docker-compose.integration.yaml "$@"
}
cleanup() {
    result=$?
    trap - EXIT INT TERM
    if [ "$result" -ne 0 ]; then
        compose logs --no-color || true
    fi
    compose down --volumes --remove-orphans || result=1
    if [ "$result" -ne 0 ]; then
        exit 1
    fi
    exit 0
}
trap cleanup EXIT
trap 'exit 1' INT TERM
compose up --build --abort-on-container-exit --exit-code-from tests || exit 1
