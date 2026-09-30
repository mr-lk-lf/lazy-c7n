# Handoff — read this first in a new session

Everything decided so far is in `docs/SPEC.md`; this file is the short "where we are / what's next".

## Roles
- The user is the **PM and final tester** (heavy c7n user, not a Go developer): they steer, Claude implements. Priorities, in order: stability, code simple enough to maintain by hand one day, a rich and pleasant UI.

## Decisions already made (don't re-litigate)
- lazy-c7n is a fully independent project: TUI, fully open, non-profit, released fast. It has no relationship with any other project.
- lazy-c7n is allowed to *execute* policies. Responsibility for what runs lies with the user.
- Safety: dry-run by default, typed confirmation for live runs, destructive-action highlighting, "not affiliated with Cloud Custodian/CNCF" disclaimer, no-warranty license.
- **Stack: Go + Bubble Tea v2 / Lip Gloss v2 / Bubbles v2** (switched from Rust/ratatui on 2026-09-30; rationale in SPEC §7).
- Repo `lazy-c7n`, binary `lazyc7n` (SPEC §9.1 still wants a collision check before publishing).
- License: `MIT OR Apache-2.0` (`LICENSE-MIT` + `LICENSE-APACHE`, the latter from apache.org).
- GitHub repo `vstrofago/lazy-c7n`, **private** until the first public release.

## Environment
- Go 1.27.1 (mise). `custodian` 0.9.52 in `.venv/`, moto 5.2.3 in `.venv-emu/` (both git-ignored; recreate per `CLAUDE.md`).
- Docker daemon runs but the user is not in the `docker` group, so Floci and the docker backend are untested. Fix: `sudo usermod -aG docker $USER` + re-login.

## Done
- **M0 steps 1–3, 5.**
  - c7n **[verify]** items resolved against 0.9.52 + moto (SPEC §3, §6.5); only the docker backend image/paths remain. Real captures in `tests/fixtures/real/c7n-0.9.52-moto/`, reproducible with `tests/fixtures/tools/capture-real.sh`.
  - Key c7n findings: `execution.end_time` (not `end`); multi-region output is `<out>/<region>/<policy>/`; `resources.json` absent on error and on live runs of non-pull modes; `action-<name>` files optional; `run` exits 2 on any policy error; logs only on stderr; c7n's resource cache (`-f`, 15 min) is shared between dry-run and live; `--dryrun` is safe for every mode, a live non-pull run provisions Lambda.
  - Go scaffold: `cmd/lazyc7n`, `internal/{app,config,ui}`; config (user + `.lazyc7n.toml`, invalid safety values fail closed), five empty screens with tabs, DRY/LIVE badge, key help, light/dark theme; tests for config, `Update` and `View`; CI (gofmt, tidy, vet, golangci-lint with `exhaustive`, tests on 3 OSes).

## Next
1. **M0 step 4: the live-run gate state machine** (SPEC §6) in `internal/app`, with table-driven tests that try to bypass it (wrong name, Esc, focus changes, a second `R`, config weakening). Needs PM input on the UX: exact wording, whether `yes-no` mode is allowed at all in v0, how non-pull "deploys infrastructure" confirmation looks.
2. **M1 — read-only browser**: policy discovery (SPEC §9.4), YAML library choice (§9.2), Policies tree + YAML view + action highlighting, Runs/Resources screens reading an existing `-s` dir (`lazyc7n -output <dir>`), using the real fixtures. First release candidate.
3. Write `docs/manual-testing.md` (the PM's test checklist against moto/Floci).
