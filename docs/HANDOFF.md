# Handoff — read this first in a new session

Everything decided so far is in `docs/SPEC.md`; this file is the short "where we are / what's next".
For the user (Spanish, with an opening prompt to paste): `docs/NEXT-STEPS.txt`.

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
- On the PM's machine the user is not in the `docker` group (fix: `sudo usermod -aG docker $USER` + re-login). Docker, Floci and the docker backend were verified on 2026-09-30 from a Claude Code cloud container (start `dockerd` as a background task there).

## Done
- **M0 complete (steps 1–5).**
  - All c7n **[verify]** items resolved against 0.9.52 (SPEC §3, §6.5), including the docker backend (SPEC §3 "Invocation backends": run with `--user <uid>:<gid>`, mount outside `/home/custodian`, always pass `-f`). Real captures against moto **and Floci** in `tests/fixtures/real/c7n-0.9.52-{moto,floci}/`, reproducible with `tests/fixtures/tools/capture-real.sh`.
  - Key c7n findings: `execution.end_time` (not `end`); multi-region output is `<out>/<region>/<policy>/`; `resources.json` absent on error and on live runs of non-pull modes; `action-<name>` files optional; `run` exits 2 on any policy error; logs only on stderr; c7n's resource cache (`-f`, 15 min) is shared between dry-run and live; `--dryrun` is safe for every mode, a live non-pull run provisions Lambda + its EventBridge rule (seen completely on Floci).
  - Go scaffold: `cmd/lazyc7n`, `internal/{app,config,ui,c7n}`; config (user + `.lazyc7n.toml`, invalid safety values fail closed), five empty screens with tabs, DRY/LIVE badge, key help, light/dark theme; tests for config, `Update` and `View`; CI (gofmt, tidy, vet, golangci-lint with `exhaustive`, tests on 3 OSes).
  - **Live-run gate** (step 4, SPEC §6.4–6.5): `internal/app/gate.go` + `gate_test.go` (table of bypass attempts, random key mashing, frozen request, config weakening). Mutation-checked: accepting a prefix, skipping DEPLOY or ignoring case each make tests fail. Action classification in `internal/c7n/safety.go`. PM decisions (2026-09-30): type the name (1 policy) or the count (several); `yes-no` rejected at startup in v0; non-pull policies need a second step typing `DEPLOY`.
  - `startLiveRun` in `internal/app/app.go` is the only place a live run starts; today it only reports back (the runner arrives in M2/M3). `Model.selected` stays empty until M1, so in the real app `R` says "select a policy first".

## Next
1. **M1 — read-only browser**: policy discovery (SPEC §9.4), YAML library choice (§9.2), Policies tree + YAML view + action highlighting (reuse `c7n.ClassifyAction`), filling `Model.selected` (with `c7n.Policy`); Runs/Resources screens reading an existing `-s` dir (`lazyc7n -output <dir>`), using the real fixtures. First release candidate.
2. Write `docs/manual-testing.md` (the PM's test checklist against moto/Floci), including walking through the live-run gate once M1 can select policies.
