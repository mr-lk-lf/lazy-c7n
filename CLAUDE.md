# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## State of the repo

Pre-alpha: only docs exist (no `Cargo.toml` yet). The source of truth is `docs/SPEC.md`; the next steps are in `docs/HANDOFF.md`. Read both before doing anything.

## What this is

`lazy-c7n` is a Rust/[ratatui](https://ratatui.rs/) TUI that wraps the Cloud Custodian (`custodian`) CLI: browse policies, validate, dry-run, run, inspect runs/logs. Sibling project `../cc-visualizer` (web dashboard, read-only) is a separate product; do not merge concerns between them.

## Non-negotiables

- Never reimplement c7n logic; always shell out to `custodian` (or its docker/command backend).
- Dry-run is the default. The live-run confirmation gate (SPEC §6) must not be bypassable; any change touching it needs a state-machine test in `update()`.
- Never store, log or render credentials/secrets.
- Keep the README disclaimer (independent project, no affiliation with Cloud Custodian/CNCF, no warranty).
- Parsing of c7n policy/output must be lenient (ignore unknown fields).
- Items tagged **[verify]** in the spec were written from memory: confirm against a real `custodian` install before depending on them, then update the spec.
- Toolchain/versions: don't guess crate versions; use `cargo add`.

## Commands

None yet. Once scaffolded (M0) document here: build, run, `cargo test`, single test, fmt/clippy.
