#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
HELM=${HELM:-helm}
"$HELM" lint --strict deployments/calendar-chart
CALENDAR_HELM="$HELM" go test -count=1 -v ./deployments/charttest
