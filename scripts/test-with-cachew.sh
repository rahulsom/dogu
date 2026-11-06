#!/usr/bin/env bash
# Run a command with one Cachew instance and a reusable on-disk cache.
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cachew_state="${CACHEW_STATE_DIR:-$repo_root/build/cachew}"
mkdir -p "$cachew_state"
cachew_state="$(cd "$cachew_state" && pwd)"
cachew_container=""
cleanup() {
  result=$?
  trap - EXIT
  if [[ -n "$cachew_container" ]]; then
    if [[ "$result" -ne 0 ]]; then docker logs "$cachew_container" >&2 || true; fi
    # Flush/close the disk metadata database before Actions saves the directory.
    docker stop --time 30 "$cachew_container" >/dev/null || true
    docker rm "$cachew_container" >/dev/null || true
  fi
  exit "$result"
}
trap cleanup EXIT
cachew_container="$(docker run -d \
  --user "$(id -u):$(id -g)" \
  -e USER=cachew \
  -p 127.0.0.1::8080 \
  -v "$repo_root/testdata/cachew.hcl:/app/cachew.hcl:ro" \
  -v "$cachew_state:/var/cache/cachew" \
  ghcr.io/block/cachew:latest --config /app/cachew.hcl)"
cachew_address="$(docker port "$cachew_container" 8080/tcp)"
export CACHEW_URL="http://$cachew_address"
ready=0
for ((attempt=0; attempt<30; attempt++)); do
  if curl --fail --silent --max-time 1 "$CACHEW_URL/_liveness" >/dev/null; then
    ready=1
    break
  fi
  sleep 1
done
if [[ "$ready" -ne 1 ]]; then
  echo "Cachew failed to become ready at $CACHEW_URL" >&2
  exit 1
fi
export DOGU_INTEGRATION=1
if [[ "$#" -eq 0 ]]; then
  set -- go test -race -cover ./...
fi
"$@"
