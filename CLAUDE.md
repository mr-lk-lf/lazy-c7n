# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## State of the repo

M0 scaffold in place (Go + Bubble Tea: config loading, empty screens, pure `Update`). The source of truth is `docs/SPEC.md`; the next steps are in `docs/HANDOFF.md`. Read both before doing anything.

## What this is

`lazy-c7n` is a Go TUI built on [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) + Lip Gloss + Bubbles that wraps the Cloud Custodian (`custodian`) CLI: browse policies, validate, dry-run, run, inspect runs/logs.

## Non-negotiables

- Never reimplement c7n logic; always shell out to `custodian` (or its docker/command backend).
- Dry-run is the default. The live-run confirmation gate (SPEC §6) must not be bypassable; any change touching it needs a state-machine test that drives `Update()` with messages.
- Never store, log or render credentials/secrets.
- Keep the README disclaimer (independent project, no affiliation with Cloud Custodian/CNCF, no warranty).
- Parsing of c7n policy/output must be lenient (ignore unknown fields).
- Items tagged **[verify]** in the spec were written from memory: confirm against a real `custodian` install before depending on them, then update the spec.
- Toolchain/versions: don't guess dependency versions; use `go get <module>@latest` (Charm v2 modules live under `charm.land/...`). Check real APIs in the module cache (`$(go env GOMODCACHE)`), Bubble Tea v2 differs a lot from v1.
- Every `switch` over an enum-like type must list all values (the `exhaustive` linter enforces it); don't add `default:` to dodge it.
- The user is the PM and final tester, not a Go developer: keep code plain and idiomatic and avoid clever abstractions.

## Commands

Go 1.26+ (installed via mise).

- Build / run: `go build ./cmd/lazyc7n`, `go run ./cmd/lazyc7n [-config <file>]`
- Tests: `go test ./...` (CI adds `-race`); single test: `go test ./internal/config -run TestInvalidSafetyValueFailsClosed`
- Lint (same as CI): `gofmt -l .` (must print nothing), `go mod tidy -diff`, `go vet ./...`, `mise x golangci-lint@2.14.0 -- golangci-lint run ./...`
- Look at the real UI without a terminal: `tmux -L t new -d -s t -x 90 -y 14 ./lazyc7n; tmux -L t capture-pane -p -t t` (send keys with `tmux -L t send-keys -t t Tab`)

c7n and the local AWS emulator (never a real account for dev work):

- `custodian` 0.9.52 lives in `.venv/` (`python3 -m venv .venv && .venv/bin/pip install c7n`).
- moto server in `.venv-emu/` (`.venv-emu/bin/pip install 'moto[server]'`); start with `.venv-emu/bin/moto_server -p 5055`. Floci (`floci/floci`, port 4566) should work too but is untested (needs Docker).
- Point `custodian` at the emulator only through the isolated env in `tests/fixtures/tools/capture-real.sh` (fake creds, `AWS_CONFIG_FILE=/dev/null`, `AWS_ENDPOINT_URL`), and pass `-f <tmp cache>` so the shared `~/.cache/cloud-custodian.cache` is not used.
- Re-capture real output fixtures: `tests/fixtures/tools/capture-real.sh` (writes `tests/fixtures/real/c7n-<version>-<emulator>/`).
