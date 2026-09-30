# Handoff — read this first in the new session

Created from an initial planning session (2026-09-30). Everything decided so far is in `docs/SPEC.md`; this file is the short "where we are / what's next".

## Decisions already made (don't re-litigate)
- lazy-c7n is a fully independent project: TUI, fully open, non-profit, released fast. It has no relationship with any other project.
- lazy-c7n is allowed to *execute* policies. Responsibility for what runs lies with the user.
- Safety choices: permissive license with no-warranty clause, dry-run by default + typed confirmation for live runs + destructive-action highlighting, and an explicit "not affiliated with Cloud Custodian/CNCF" disclaimer.
- Repo name `lazy-c7n`; proposed binary/crate name `lazyc7n` (open question, SPEC §9.1).
- License: `MIT OR Apache-2.0` (`LICENSE-MIT` + `LICENSE-APACHE`, the latter from apache.org).

## Environment notes (as of creation)
- This machine had **no Rust toolchain and no `custodian`** installed. Install with mise/rustup (`mise use -g rust` or rustup) and `python3 -m venv .venv && .venv/bin/pip install c7n`.
- Git: GitHub repo `vstrofago/lazy-c7n`, **private** until the first public release; `main` pushed.

## Progress (2026-09-30, session 2)
- Rust 1.98.1 (rustup, `~/.cargo/bin`), `custodian` 0.9.52 in `.venv/`, moto 5.2.3 in `.venv-emu/`.
- Docker daemon is running but the user is not in the `docker` group, so Floci and the docker backend could not be tried. Fix: `sudo usermod -aG docker $USER` + re-login.
- All **[verify]** items resolved against c7n 0.9.52 + moto (SPEC §3, §6.5), except the docker backend image/paths. Captures in `tests/fixtures/real/c7n-0.9.52-moto/`, reproducible via `tests/fixtures/tools/capture-real.sh`.
- Findings that change the design: `execution.end_time` (not `end`); multi-region output is `<out>/<region>/<policy>/`; `resources.json` absent on error and on live runs of non-pull modes; `action-<name>` files are optional; `run` exits 2 on any policy error; logs only on stderr; c7n's resource cache (`-f`, 15 min) is shared between dry-run and live.
- M0 steps 1, 2, 3 done: `cargo init` (crate/binary `lazyc7n`), ratatui 0.30 via `ratatui::run` (panic hook restores terminal), config loading with user + `.lazyc7n.toml` merge, empty screens, CI (fmt, clippy, test on 3 OSes). Dual license files in place (`LICENSE-MIT`, `LICENSE-APACHE` from apache.org).
- Next: step 4 (live-run gate state machine + its tests), then M1. No `tokio` yet: add it with the runner in M2.

## Suggested first steps (M0, see SPEC §8)
1. Install Rust toolchain; install c7n in `.venv` (git-ignored).
2. Resolve every **[verify]** item: run `custodian run -h`, `custodian validate -h`, `custodian schema --json`, run a harmless policy with `--dryrun -s out/` against fixtures/or a free-tier/empty account (or a `mode: pull` policy that matches nothing), and record the real output tree under `tests/fixtures/real/` (scrubbed). Update SPEC accordingly. Specifically confirm: what `--dryrun` does for non-`pull` modes; the exact file set in `<out>/<policy>/`.
3. `cargo init` (binary crate), add deps with `cargo add`, set up CI (fmt, clippy, test), terminal setup/teardown with panic hook, empty Policies screen reading a config file.
4. Write the first real test: the live-run gate state machine (SPEC §6/§11).
5. Update `CLAUDE.md` "Commands" section.

## Suggested opening prompt for the new session
> Lee `CLAUDE.md`, `docs/SPEC.md` y `docs/HANDOFF.md`. Empecemos M0: instala el toolchain de Rust y c7n en un venv, resuelve los puntos **[verify]** del spec contra un `custodian` real, y luego scaffoldea el proyecto con `cargo init`. Antes de escribir código, usa el skill de brainstorming solo si hay alguna decisión abierta de SPEC §9 que bloquee.

Open the new session with: `cd ~/Documents/harder-better-faster-stronger/10-Projects/lazy-c7n && claude`
