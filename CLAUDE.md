# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## State of the repo

Pre-alpha, M0 scaffold in place (config loading, empty screens, pure `App::update`). The source of truth is `docs/SPEC.md`; the next steps are in `docs/HANDOFF.md`. Read both before doing anything.

## What this is

`lazy-c7n` is a Rust/[ratatui](https://ratatui.rs/) TUI that wraps the Cloud Custodian (`custodian`) CLI: browse policies, validate, dry-run, run, inspect runs/logs.

## Non-negotiables

- Never reimplement c7n logic; always shell out to `custodian` (or its docker/command backend).
- Dry-run is the default. The live-run confirmation gate (SPEC §6) must not be bypassable; any change touching it needs a state-machine test in `update()`.
- Never store, log or render credentials/secrets.
- Keep the README disclaimer (independent project, no affiliation with Cloud Custodian/CNCF, no warranty).
- Parsing of c7n policy/output must be lenient (ignore unknown fields).
- Items tagged **[verify]** in the spec were written from memory: confirm against a real `custodian` install before depending on them, then update the spec.
- Toolchain/versions: don't guess crate versions; use `cargo add`.

## Commands

Rust lives in `~/.cargo/bin` (rustup); if `cargo` is not found, `export PATH="$HOME/.cargo/bin:$PATH"`.

- Build / run: `cargo build`, `cargo run -- [--config <file>]`
- Tests: `cargo test`; single test: `cargo test <name_substring>` (e.g. `cargo test config::tests::explicit_file_is_loaded`)
- Lint (same as CI): `cargo fmt --all --check` and `cargo clippy --all-targets -- -D warnings`

c7n and the local AWS emulator (never a real account for dev work):

- `custodian` 0.9.52 lives in `.venv/` (`python3 -m venv .venv && .venv/bin/pip install c7n`).
- moto server in `.venv-emu/` (`.venv-emu/bin/pip install 'moto[server]'`); start with `.venv-emu/bin/moto_server -p 5055`. Floci (`floci/floci`, port 4566) should work too but is untested (needs Docker).
- Point `custodian` at the emulator only through the isolated env in `tests/fixtures/tools/capture-real.sh` (fake creds, `AWS_CONFIG_FILE=/dev/null`, `AWS_ENDPOINT_URL`), and pass `-f <tmp cache>` so the shared `~/.cache/cloud-custodian.cache` is not used.
- Re-capture real output fixtures: `tests/fixtures/tools/capture-real.sh` (writes `tests/fixtures/real/c7n-<version>-<emulator>/`).
