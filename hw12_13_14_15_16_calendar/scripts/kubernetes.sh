#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
PROFILE=${PROFILE:-calendar-hw16}
KUBE_CONTEXT=${KUBE_CONTEXT:-$PROFILE}
case "$PROFILE" in calendar-hw16|calendar-hw16-*) ;; *) echo 'Refusing profile outside calendar-hw16' >&2; exit 1;; esac
if [ "$KUBE_CONTEXT" != "$PROFILE" ]; then
    echo "Refusing context $KUBE_CONTEXT: expected isolated profile $PROFILE" >&2
    exit 1
fi
STATE=${CALENDAR_K8S_STATE:-$PWD/.local/hw16}
mkdir -p "$STATE"
export MINIKUBE_HOME="$STATE/minikube"
export KUBECONFIG="$STATE/kubeconfig"
MINIKUBE=${MINIKUBE:-minikube}
KUBECTL=${KUBECTL:-kubectl}
HELM=${HELM:-helm}
NAMESPACE=${NAMESPACE:-calendar-hw16}
RELEASE=${RELEASE:-calendar}
for name in "$PROFILE" "$NAMESPACE" "$RELEASE"; do
    case "$name" in ''|*[!a-z0-9-]*|-*|*-) echo "Invalid Kubernetes name: $name" >&2; exit 1;; esac
    [ ${#name} -le 40 ] || { echo 'Names must be at most 40 characters' >&2; exit 1; }
done
kube() { "$KUBECTL" --kubeconfig "$KUBECONFIG" --context "$KUBE_CONTEXT" "$@"; }
helm_local() { "$HELM" --kubeconfig "$KUBECONFIG" --kube-context "$KUBE_CONTEXT" "$@"; }
chart=deployments/calendar-chart
diagnostics() {
    kube -n "$NAMESPACE" get pods,events || true
    kube -n "$NAMESPACE" logs -l app.kubernetes.io/instance="$RELEASE" --all-containers --tail=30 --prefix || true
}
case "${1:-status}" in
    start)
        "$MINIKUBE" -p "$PROFILE" start --driver=docker --cpus=2 --memory=4096 \
            --kubernetes-version="${KUBERNETES_VERSION:-v1.35.0}" --keep-context
        "$MINIKUBE" -p "$PROFILE" addons enable ingress
        kube wait --for=condition=Ready node --all --timeout=300s
        ;;
    stop) "$MINIKUBE" -p "$PROFILE" stop ;;
    status)
        kube get nodes
        kube -n "$NAMESPACE" get deployments,statefulsets,pods,services,ingress,pvc
        ;;
    images)
        for component in api scheduler sender integration; do
            suffix=.$component
            [ "$component" != api ] || suffix=''
            docker build -f "build/Dockerfile$suffix" -t "calendar-$component:hw16" .
            "$MINIKUBE" -p "$PROFILE" image load "calendar-$component:hw16"
        done
        for image in postgres:17.6-alpine rabbitmq:4.1.4-alpine; do
            if docker image inspect "$image" >/dev/null 2>&1; then
                "$MINIKUBE" -p "$PROFILE" image load "$image"
            fi
        done
        ;;
    deploy)
        if ! helm_local upgrade --install "$RELEASE" "$chart" --namespace "$NAMESPACE" \
            --create-namespace --wait --timeout=600s; then
            diagnostics
            exit 1
        fi
        ;;
    undeploy) helm_local uninstall "$RELEASE" --namespace "$NAMESPACE" --wait --timeout=180s ;;
    ingress)
        echo 'HTTP: curl -H "Host: calendar.local" http://127.0.0.1:18888/hello'
        kube -n ingress-nginx port-forward service/ingress-nginx-controller 18888:80
        ;;
    api) kube -n "$NAMESPACE" port-forward "service/$RELEASE-api" 18888:http 15051:grpc ;;
    smoke)
        kube -n "$NAMESPACE" delete job "$RELEASE-integration" --ignore-not-found --wait=true
        umask 077
        smoke_dir=$(mktemp -d "$STATE/smoke.XXXXXX")
        trap 'rm -rf "$smoke_dir"' EXIT
        trap 'exit 1' INT TERM
        helm_local get values "$RELEASE" --namespace "$NAMESPACE" --all -o yaml > "$smoke_dir/values.yaml"
        set -- --set tests.enabled=true
        if [ "${SMOKE_API_URL+x}" = x ]; then
            set -- "$@" --set-string "tests.apiURL=$SMOKE_API_URL"
        fi
        helm_local template "$RELEASE" "$chart" --namespace "$NAMESPACE" -f "$smoke_dir/values.yaml" \
            "$@" --show-only templates/integration-job.yaml > "$smoke_dir/job.yaml"
        kube -n "$NAMESPACE" apply -f "$smoke_dir/job.yaml"
        result=1
        attempt=0
        while [ "$attempt" -lt 120 ]; do
            conditions=$(kube -n "$NAMESPACE" get job "$RELEASE-integration" -o 'jsonpath={.status.conditions[*].type}')
            case "$conditions" in
                *Complete*) result=0; break ;;
                *Failed*) break ;;
            esac
            attempt=$((attempt + 1))
            sleep 2
        done
        kube -n "$NAMESPACE" logs "job/$RELEASE-integration" --all-containers || result=1
        if [ "$result" -ne 0 ]; then diagnostics; fi
        exit "$result"
        ;;
    *) echo 'Usage: kubernetes.sh start|stop|status|images|deploy|undeploy|ingress|api|smoke' >&2; exit 1 ;;
esac
