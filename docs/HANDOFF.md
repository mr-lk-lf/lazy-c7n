# Handoff — read this first in the new session

Created from a planning session in `../cc-visualizer` (2026-09-30). Everything decided so far is in `docs/SPEC.md`; this file is the short "where we are / what's next".

## Decisions already made (don't re-litigate)
- Two separate projects: **cc-visualizer** (web, read-only, aims at being an open alternative to Stacklet/Harness Cloud Asset Governance; proceeds slowly; funding only for the heavy features later) and **lazy-c7n** (this repo; TUI, fully open, non-profit, released fast).
- lazy-c7n is allowed to *execute* policies (unlike cc-visualizer, whose philosophy is strictly read-only). Responsibility for what runs lies with the user.
- Safety choices: permissive license with no-warranty clause, dry-run by default + typed confirmation for live runs + destructive-action highlighting, and an explicit "not affiliated with Cloud Custodian/CNCF" disclaimer.
- Repo name `lazy-c7n`; proposed binary/crate name `lazyc7n` (open question, SPEC §9.1).
- License: MIT committed now; target is `MIT OR Apache-2.0` (add `LICENSE-APACHE` from the canonical text at scaffold time, not from memory).
- cc-visualizer is to be left alone for now (known issues there: broken lint targets, README mentions non-existent files; license still TBD).

## Environment notes (as of creation)
- This machine had **no Rust toolchain and no `custodian`** installed. Install with mise/rustup (`mise use -g rust` or rustup) and `python3 -m venv .venv && .venv/bin/pip install c7n`.
- Git: repo initialised on `main`, one local commit, **no remote, nothing pushed**. `gh` CLI is available; creating the GitHub repo was deliberately left for the user to decide (name/visibility).

## Suggested first steps (M0, see SPEC §8)
1. Install Rust toolchain; install c7n in `.venv` (git-ignored).
2. Resolve every **[verify]** item: run `custodian run -h`, `custodian validate -h`, `custodian schema --json`, run a harmless policy with `--dryrun -s out/` against fixtures/or a free-tier/empty account (or a `mode: pull` policy that matches nothing), and record the real output tree under `tests/fixtures/real/` (scrubbed). Update SPEC accordingly. Specifically confirm: what `--dryrun` does for non-`pull` modes; the exact file set in `<out>/<policy>/`.
3. `cargo init` (binary crate), add deps with `cargo add`, set up CI (fmt, clippy, test), terminal setup/teardown with panic hook, empty Policies screen reading a config file.
4. Write the first real test: the live-run gate state machine (SPEC §6/§11).
5. Update `CLAUDE.md` "Commands" section.

## Suggested opening prompt for the new session
> Lee `CLAUDE.md`, `docs/SPEC.md` y `docs/HANDOFF.md`. Empecemos M0: instala el toolchain de Rust y c7n en un venv, resuelve los puntos **[verify]** del spec contra un `custodian` real, y luego scaffoldea el proyecto con `cargo init`. Antes de escribir código, usa el skill de brainstorming solo si hay alguna decisión abierta de SPEC §9 que bloquee.

Open the new session with: `cd ~/Documents/harder-better-faster-stronger/10-Projects/lazy-c7n && claude`
