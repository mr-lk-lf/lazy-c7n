#!/usr/bin/env bash
# Capture real `custodian` output against a LOCAL AWS emulator into
# tests/fixtures/real/c7n-<version>-<emulator>/. See tests/fixtures/real/README.md.
#
# Prereqs: .venv with c7n, .venv-emu with moto[server] (or Floci on :4566),
# emulator already running. Usage:
#   .venv-emu/bin/moto_server -p 5055 &
#   tests/fixtures/tools/capture-real.sh            # moto on :5055
#   LC7N_EMU_URL=http://localhost:4566 LC7N_EMU_NAME=floci tests/fixtures/tools/capture-real.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
TOOLS="$ROOT/tests/fixtures/tools"
CUSTODIAN="${CUSTODIAN:-$ROOT/.venv/bin/custodian}"
PY="${LC7N_EMU_PY:-$ROOT/.venv-emu/bin/python}"
EMU_URL="${LC7N_EMU_URL:-http://localhost:5055}"
EMU_NAME="${LC7N_EMU_NAME:-moto}"

# Hard isolation from any real AWS account: no config/credential files, fake keys,
# every API call goes to the local endpoint.
export AWS_CONFIG_FILE=/dev/null AWS_SHARED_CREDENTIALS_FILE=/dev/null
unset AWS_PROFILE AWS_SESSION_TOKEN AWS_SECURITY_TOKEN
export AWS_ACCESS_KEY_ID=testing AWS_SECRET_ACCESS_KEY=testing AWS_DEFAULT_REGION=us-east-1
export AWS_ENDPOINT_URL="$EMU_URL" AWS_MAX_ATTEMPTS=1

VERSION="$("$CUSTODIAN" version)"
DEST="$ROOT/tests/fixtures/real/c7n-$VERSION-$EMU_NAME"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

if [ "$EMU_NAME" = moto ]; then
  curl -fsS -X POST "$EMU_URL/moto-api/reset" >/dev/null
fi
"$PY" "$TOOLS/seed_emulator.py"

cp "$TOOLS/policies/"*.yml "$WORK/"
cd "$WORK"

# scenario <name> <custodian args...>: records argv, exit code, stdout, stderr and
# the -s output dir (if any) under $WORK/scenarios/<name>/.
scenario() {
  local name="$1"; shift
  local dir="scenarios/$name"
  mkdir -p "$dir"
  printf '%s\n' "custodian $*" >"$dir/argv.txt"
  set +e
  "$CUSTODIAN" "$@" >"$dir/stdout.txt" 2>"$dir/stderr.txt"
  echo $? >"$dir/exit_code.txt"
  set -e
  if [ -d out ]; then mv out "$dir/out"; fi
}

# Private resource cache per capture, so results never come from ~/.cache.
CACHE=(-f "$WORK/c7n.cache")

scenario validate-ok       validate policies.yml
scenario validate-invalid  validate invalid.yml
scenario dryrun            run "${CACHE[@]}" --dryrun -s out policies.yml
scenario dryrun-glob       run "${CACHE[@]}" --dryrun -s out -p 's3-*' policies.yml
scenario dryrun-multiregion run "${CACHE[@]}" --dryrun -s out -r us-east-1 -r eu-west-1 -p 'ec2-*' policies.yml
scenario live              run "${CACHE[@]}" -s out -p s3-untagged-owner -p ec2-mark-stop policies.yml
scenario live-periodic     run "${CACHE[@]}" -s out -p s3-periodic policies.yml
AWS_ENDPOINT_URL=http://127.0.0.1:9 \
scenario dryrun-api-error  run -f "$WORK/c7n-down.cache" --dryrun -s out -p ec2-none-match policies.yml
scenario report-json       report -s scenarios/dryrun/out --format json -p s3-untagged-owner policies.yml
scenario report-csv        report -s scenarios/dryrun/out --format csv -p ec2-mark-stop policies.yml

# Scrub machine-specific paths.
grep -rlZ -e "$ROOT" -e "$WORK" -e "$HOME" scenarios 2>/dev/null | xargs -0 -r sed -i \
  -e "s#$WORK#<work>#g" -e "s#$ROOT#<repo>#g" -e "s#$HOME#~#g"

rm -rf "$DEST"
mkdir -p "$DEST"
mv scenarios/* "$DEST/"
cp policies.yml invalid.yml "$DEST/"
echo "captured into ${DEST#"$ROOT"/}"
