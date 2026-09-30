#!/usr/bin/env bash
# Try lazyc7n against a LOCAL AWS emulator (Floci), never a real account.
#
#   scripts/floci-dev.sh            # start Floci, seed it, open lazyc7n on examples/policies
#   scripts/floci-dev.sh --docker   # same, but run custodian with the docker backend
#   scripts/floci-dev.sh --reset    # recreate the Floci container (fresh resources)
#
# Needs: Docker, .venv with c7n and .venv-emu with boto3 (see CLAUDE.md).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PORT=4566
BACKEND=binary
for arg in "$@"; do
  case "$arg" in
    --docker) BACKEND=docker ;;
    --reset) docker rm -f lazyc7n-floci >/dev/null 2>&1 || true ;;
    *) echo "unknown option $arg" >&2; exit 2 ;;
  esac
done

if ! docker ps --format '{{.Names}}' | grep -qx lazyc7n-floci; then
  docker rm -f lazyc7n-floci >/dev/null 2>&1 || true
  docker run -d --name lazyc7n-floci -p "$PORT:4566" floci/floci:latest >/dev/null
  echo "waiting for Floci on :$PORT..."
  for _ in $(seq 1 60); do
    curl -fsS "localhost:$PORT/_localstack/health" >/dev/null 2>&1 && break
    sleep 1
  done
  SEED=1
fi

# Hard isolation from any real AWS account (same as tests/fixtures/tools/capture-real.sh).
export AWS_CONFIG_FILE=/dev/null AWS_SHARED_CREDENTIALS_FILE=/dev/null
unset AWS_PROFILE AWS_SESSION_TOKEN AWS_SECURITY_TOKEN
export AWS_ACCESS_KEY_ID=testing AWS_SECRET_ACCESS_KEY=testing AWS_DEFAULT_REGION=us-east-1
export AWS_ENDPOINT_URL="http://localhost:$PORT" AWS_MAX_ATTEMPTS=1

if [ "${SEED:-}" = 1 ]; then
  "$ROOT/.venv-emu/bin/python" "$ROOT/scripts/seed_demo.py"
fi

STATE="$ROOT/.dev-state"
mkdir -p "$STATE"
CONFIG="$STATE/config.toml"
if [ "$BACKEND" = docker ]; then
  # Inside the container localhost is the container itself: use the host network.
  cat >"$CONFIG" <<TOML
policy_dirs = ["$ROOT/examples/policies"]
state_dir = "$STATE/docker"
[runner]
kind = "docker"
docker_args = ["--network", "host"]
TOML
else
  cat >"$CONFIG" <<TOML
policy_dirs = ["$ROOT/examples/policies"]
state_dir = "$STATE/binary"
[runner]
kind = "binary"
custodian = "$ROOT/.venv/bin/custodian"
TOML
fi

cd "$ROOT"
go build -o "$STATE/lazyc7n" ./cmd/lazyc7n
exec "$STATE/lazyc7n" -config "$CONFIG"
