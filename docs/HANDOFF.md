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
- **M0** (2026-09-30): c7n 0.9.52 verified against moto and Floci (SPEC §3, §6.5, fixtures in `tests/fixtures/real/`), docker backend verified, Go scaffold + CI, live-run gate with PM decisions (type the name / the count; no `yes-no` in v0; `DEPLOY` step for non-pull modes).
- **M1** read-only browser: policy discovery + lenient YAML parsing with line numbers (`go.yaml.in/yaml/v3`), Policies tree/filter/selection/highlighted YAML, Runs/Resources from `-output <dir>`.
- **M2** runner (binary/command/docker argv, process group, streaming, cancel), run store (`run.json`, retention), `v` validate, `d` dry-run, Jobs screen, run history, custodian version in the header.
- **M3** live runs: the gate freezes the exact spec; `startLiveRun` (only called on approval) starts it. `c7n.Select` refuses selections where custodian's `-p` globs would run more than what was chosen.
- **M4** Schema browser (cached `schema --json`, help per action/filter), `e` → `$EDITOR` at the line, `-prune`, `theme`.
- Everything checked end-to-end against Floci from the TUI (binary and docker backends): dry-runs, a live `mark-for-op` (tags appear), a live `periodic` policy (Lambda + EventBridge deployed, status `deployed`).
- After M4 (PM requests, 2026-10-01): colour themes (`theme` + `appearance`, `T` cycles; lazyc7n, terminal = ANSI palette of the terminal, catppuccin, gruvbox, everforest, tokyonight, dracula, each with light/dark variants except dracula) and a styled Resources view built on `custodian report --format csv` (table with c7n's default columns + resource card; raw JSON on `t`).
- Run summary in Runs (`c7n.Summarize`): matches per type/region, matches hit by destructive / changing actions, failed and deployed policies; ⚠ in the run list; ACTIONS column.
- 2026-10-01: `lazyc7n version`; `[safety.actions]` overrides (stricter only for built-in destructive); streamed `resources.json` capped at 10,000 (render of 10k rows ~3 ms); target context (profile/region/endpoint) in the status bar and gate; release tooling (GoReleaser validated with a snapshot, release workflow on tags, checksum-verified `install.sh`, govulncheck in CI, Dependabot, third-party licences in archives). See `docs/RELEASING.md`.
- Dependencies (2026-10-01): 21 modules compiled in, all MIT / BSD-3 / (yaml) MIT+Apache-2.0, no cgo, no network or crypto code. govulncheck could not reach vuln.go.dev from the dev container; it now runs in CI.
- Work is on branch `tui-m0-m4` (pushed, not merged to `main`).
- Tests: `go test -race ./...` green; lint clean; cross-builds for Windows/macOS. Gate mutation-checked (see commit messages).

## Next
1. PM: run `docs/manual-testing.md` against Floci (`scripts/floci-dev.sh`), then decide whether to merge `tui-m0-m4` into `main`.
2. Things noticed but not done: overriding action classes from the config (SPEC §6.3); showing profile/region from the environment in the status bar (SPEC §4); a pager for huge `resources.json` (today it is read whole); Windows cancel is a hard kill; key remapping.
3. **M5 — release** (not started, on hold by PM request): name collision check (SPEC §9.1), GoReleaser, `go install`, Homebrew/AUR/Scoop, demo GIF with vhs, make the repo public.
