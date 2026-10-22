#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
if KUBE_CONTEXT=remote-production sh scripts/kubernetes.sh deploy >"$scratch/output" 2>&1; then
    echo 'unsafe context was accepted' >&2
    exit 1
fi
if ! grep -q 'Refusing context' "$scratch/output"; then
    cat "$scratch/output" >&2
    echo 'missing explicit context guard' >&2
    exit 1
fi
echo 'Kubernetes context guard: PASS'
cat > "$scratch/docker" <<'FAKE'
#!/bin/sh
if [ "$1 $2" = 'image inspect' ]; then exit 1; fi
exit 0
FAKE
cat > "$scratch/minikube" <<'FAKE'
#!/bin/sh
exit 0
FAKE
chmod +x "$scratch/docker" "$scratch/minikube"
PATH="$scratch:$PATH" MINIKUBE="$scratch/minikube" CALENDAR_K8S_STATE="$scratch/state" \
    sh scripts/kubernetes.sh images
echo 'Image loading without host infrastructure cache: PASS'
cat > "$scratch/helm" <<'FAKE'
#!/bin/sh
case " $* " in
    *' get values '*) printf 'service:\n  httpPort: 18080\n';;
    *' template '*)
        values=''
        while [ "$#" -gt 0 ]; do
            if [ "$1" = '-f' ]; then shift; values=$1; fi
            shift
        done
        if [ -z "$values" ] || ! grep -q '18080' "$values"; then
            echo 'smoke ignored installed values' >&2
            exit 1
        fi
        printf 'kind: Job\n';;
esac
FAKE
cat > "$scratch/kubectl" <<'FAKE'
#!/bin/sh
case " $* " in
    *' apply '*) :;;
    *' get job '*) printf Complete;;
esac
FAKE
chmod +x "$scratch/helm" "$scratch/kubectl"
HELM="$scratch/helm" KUBECTL="$scratch/kubectl" CALENDAR_K8S_STATE="$scratch/state" \
    sh scripts/kubernetes.sh smoke >"$scratch/smoke" 2>&1
if grep -q "smoke ignored installed values" "$scratch/smoke"; then
    cat "$scratch/smoke" >&2
    exit 1
fi
echo 'Smoke uses installed Helm values: PASS'
