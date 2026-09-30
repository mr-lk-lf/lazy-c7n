# lazy-c7n — Specification (v0.1 draft)

A terminal UI (TUI) wrapper around the [Cloud Custodian](https://cloudcustodian.io/) CLI (`custodian`, a.k.a. c7n). Think `lazygit` / `lazydocker`, for c7n: browse policies, validate and dry-run them, run them, and inspect reports and logs without memorising flags.

Status legend used below: **[verify]** = written from memory/docs, must be confirmed against a real `custodian` install (`custodian run -h`, real output dirs) before being relied on.

## 1. Goals and non-goals

### Goals
- Make day-to-day c7n usage fast: list policies, validate, dry-run, run, read results and logs, all from one screen-driven tool.
- Be a thin, honest wrapper: every action maps to a visible `custodian ...` command line the user can see (and copy).
- **Safe by default**: dry-run is the default, real runs need explicit confirmation.
- Single static binary, no server, no database, no account, no telemetry.
- Fully open source, non-profit (MIT OR Apache-2.0, see §10).

### Non-goals
- Not a reimplementation of c7n. Policy evaluation, filters, actions and cloud API calls are always done by the real `custodian` executable.
- No hosting, no multi-user features, no RBAC, no central history. (That territory belongs to the sibling project `cc-visualizer`.)
- No credential management. Credentials are whatever the user's environment already provides (env vars, profiles, SSO, docker env-file). lazy-c7n never stores or prints secrets.
- No policy authoring IDE in v0.x (viewing yes; a full editor is out; "open in `$EDITOR`" is in).
- No official affiliation with Cloud Custodian or the CNCF (see §10).

## 2. Users and core workflows

Primary user: a cloud/platform/security engineer who already writes c7n YAML and runs `custodian` by hand or in CI.

1. **Browse**: open a directory of policy files → see every policy (name, resource, mode, filter/action summary).
2. **Validate**: select file/policy → `custodian validate` → errors inline.
3. **Dry-run**: select policy/policies → `custodian run --dryrun` → live log, then matched resources table.
4. **Run**: same, without `--dryrun`, behind the confirmation gate (§6).
5. **Inspect**: open past runs, view matched resources (with JSON detail), policy log, metadata.
6. **Explore schema**: `custodian schema` browser (resource → filters/actions) with help text.

## 3. Relationship to `custodian`

lazy-c7n shells out; it never links c7n code.

| Need | Command | Notes |
|---|---|---|
| Validate | `custodian validate <file>` | parse stderr/exit code |
| Dry-run | `custodian run --dryrun -s <out> [-p <policy>] [--region R] <file>` | **[verify]** flag set via `custodian run -h` |
| Run | `custodian run -s <out> ... <file>` | gated |
| Report | `custodian report -s <out> --format json <file>` | optional; we can read `resources.json` directly |
| Schema | `custodian schema [--json] [<cloud>.<resource>[.<category>.<item>]]` | cache result per c7n version |
| Version | `custodian version` | shown in status bar; cache key for schema |

### Invocation backends (configurable)
1. `binary` (default): `custodian` on `$PATH`, or an explicit path (e.g. a venv).
2. `docker`: `docker run ... cloudcustodian/c7n run -s /home/custodian/output /home/custodian/policy.yml` with env passthrough of `AWS_*`, `AZURE_*`, `GOOGLE_*` (pattern from the official quickstart). Volume mounts for policy dir and run output dir.
3. `command`: arbitrary prefix (e.g. `uvx --from c7n custodian`, `aws-vault exec prod --`).

The backend is an abstraction (`trait Runner`) so a **fake runner** can be used in tests (§11).

### c7n output layout (what we read back)
With `-s <out>` c7n writes one sub-directory per policy **[verify exact file set on a real run]**:

```
<out>/<policy-name>/
  metadata.json        # policy spec, account_id, region, execution.start/end  (written last)
  resources.json       # array of matched resources in raw provider API shape
  custodian-run.log    # per-policy log
  (other files, e.g. per-action outputs, may exist)
```

Important: c7n reuses `<out>/<policy-name>/` — running the same policy again into the same `-s` dir **overwrites** it. So lazy-c7n gives **every run its own output dir** (§5) instead of reusing one. This is what makes a local run history possible.

The same layout is what the sibling project `cc-visualizer` ingests (its `ingestor/parser.py` is a working reference for the `metadata.json` / `resources.json` fields used: `policy.name`, `policy.resource`, `region`, `account_id`, `execution.start`, `execution.end`).

## 4. Screens and navigation

Layout: lazygit-style panes. Left column = lists, right = detail/preview, bottom = key hints, top/bottom status bar (custodian version, backend, active profile/region *as read from env*, mode badge DRY/LIVE).

| Screen | Left | Right |
|---|---|---|
| **Policies** | tree: policy dir → files → policies, fuzzy filter `/` | YAML (syntax-highlighted), summary: resource, mode, filters, **actions (mutating ones highlighted)** |
| **Runs** | history (newest first): time, policy, dry/live, status, matched count | run detail: metadata, resources table, log tab, command line used |
| **Resources** | matched resources of selected run | pretty JSON of selected resource, tags |
| **Schema** | resource types → filters/actions | help text and JSON schema from `custodian schema` |
| **Jobs** | running/queued jobs | live streaming stdout/stderr |

Global keys (vim-ish + arrows; all remappable later): `j/k` move, `h/l` or `Tab` switch pane, `/` filter, `Enter` open, `Esc` back, `v` validate, `d` dry-run, `R` run (live, gated), `e` open in `$EDITOR`, `y` copy command line, `?` help, `q` quit.

## 5. Data model and state

No database. Plain files only.

- **Config** (`$XDG_CONFIG_HOME/lazyc7n/config.toml`, project-local `.lazyc7n.toml` overrides):
  ```toml
  policy_dirs = ["./policies"]
  [runner]
  kind = "binary"            # binary | docker | command
  custodian = "custodian"    # or path / venv
  # command = ["uvx", "--from", "c7n", "custodian"]
  [defaults]
  region = ""                # empty = let custodian decide
  cache_period = ""          # passthrough, optional
  [safety]
  default_dry_run = true     # changing this to false is allowed but shows a warning
  confirm_live = "type-name" # type-name | yes-no
  ```
- **State dir** (`$XDG_STATE_HOME/lazyc7n/`):
  ```
  runs/<run-id>/           # run-id = <UTC timestamp>-<short id>
    run.json               # our own record: argv, backend, dry_run, policies, started/ended, exit code, custodian version
    out/                   # passed as `-s` → c7n's native output (<policy>/metadata.json, resources.json, ...)
    stdout.log stderr.log  # raw captured streams
  schema-cache/<c7n-version>.json
  ```
- Retention: keep last N runs (default 200) and/or max age; manual prune command.
- In-memory model: Elm-style architecture (see §7): `App { screen, policies, runs, jobs, config, ... }`; all I/O is performed by tasks that send `Msg`s back.

## 6. Safety model (first-class feature)

1. **Dry-run by default.** The `d` key is dry-run; live run is a different key (`R`, shift) and a different badge colour.
2. **Pre-flight summary before any live run**: exact command line, policy names, region(s), backend, and the **actions** each policy will perform.
3. **Action classification** (static, by c7n action `type`):
   - *destructive*: `terminate`, `delete`, `stop`, `detach`, `release`, `deregister`, `remove-*`, `set-*` that remove access, etc. (list lives in code, is data-driven, and is overridable in config)
   - *mutating*: `tag`, `mark-for-op`, `modify-*`, `set-*`, …
   - *notify-only*: `notify`, `post-finding`, …
   - Unknown action types are treated as **mutating** (fail closed).
4. **Confirmation gate** for live runs: default `type-name` = user must type the policy name (or `ALL` for multi-policy). Extra red warning when any destructive action is present. Never skippable by a config key in a way that is silent; if `confirm_live` is weakened, the status bar shows it.
5. **Non-`pull` modes warning [verify behaviour]**: for `mode.type` other than `pull` (e.g. `periodic`, `cloudtrail`, `config-rule`), `custodian run` may *provision* Lambda functions/event sources rather than evaluating once. lazy-c7n flags these policies explicitly and (v0) blocks live-running them unless an "I understand this deploys infrastructure" confirmation is given.
6. **Credentials never shown**: only non-secret context (profile name, region, account id *if already present in c7n output*). Env var values matching `*KEY*|*SECRET*|*TOKEN*` are never rendered or logged.
7. **Audit trail**: every run, dry or live, has a `run.json` with the full argv (secrets redacted).
8. **No implicit live**: there is no CLI flag that skips the gate in interactive mode. (A future non-interactive `lazyc7n run --yes` for scripts, if ever added, must be explicit and documented as a scripting escape hatch.)

## 7. Architecture (Rust)

Stack (resolve exact versions with `cargo add` at scaffold time; do not guess versions):
- `ratatui` + `crossterm` (TUI); follow the official ratatui async/Elm ("The Elm Architecture") template.
- `tokio` (process supervision, streaming, file watching ticks).
- `serde`, `serde_json`; YAML via a maintained serde-compatible crate (`serde_yaml` is unmaintained — evaluate `serde_yaml_ng`/`serde_norway`/`yaml-rust2`) — only needed for *display/summary*, unknown keys must be preserved/ignored, never fail on unknown c7n fields.
- `clap` (CLI entry + future subcommands), `toml`, `directories` (XDG paths), `anyhow` + `thiserror`.
- Syntax highlighting for YAML/JSON: `syntect` or a small hand-rolled highlighter (decide after spike; keep binary size in mind).
- Testing: `insta` (snapshots) with ratatui `TestBackend`, `assert_cmd`/`tempfile` for process tests.

Module sketch:
```
src/
  main.rs            # arg parsing, terminal setup/teardown, panic hook that restores terminal
  app.rs             # App state + update(Msg) -> Vec<Cmd>
  msg.rs             # Msg / Cmd enums
  ui/                # pure render fns: fn view(&App, &mut Frame); one module per screen
  runner/            # trait Runner; binary.rs, docker.rs, command.rs, fake.rs
  c7n/
    policy.rs        # lenient policy YAML model (name, resource, mode, filters, actions)
    output.rs        # read metadata.json / resources.json / custodian-run.log
    schema.rs        # `custodian schema --json` cache + lookup
    safety.rs        # action classification + preflight summary
  store/             # runs dir management (run.json, retention)
  config.rs
```

Key design rules:
- `update()` is pure (no I/O); side effects are `Cmd`s executed by the runtime. This makes the safety gate testable as a state machine.
- Subprocesses: stream stdout/stderr line-by-line into `Msg::JobOutput`; support cancel (SIGINT first, then kill after timeout). Run in their own process group.
- Parsing must be **lenient**: c7n versions/providers differ; unknown fields ignored, missing optional fields shown as `-`.
- Large `resources.json` (100k+ items): load lazily/paged, never block the render loop.
- Multi-cloud: v0.1 targets AWS shapes first, but nothing in the core may assume AWS (Azure/GCP resource ids differ). Resource-id extraction is a pluggable table (see `cc-visualizer/ingestor/parser.py` `RESOURCE_ID_FIELDS` for a starting AWS mapping).

## 8. Milestones

- **M0 — Scaffold**: cargo project, CI (fmt, clippy, test), terminal setup/teardown w/ panic hook, empty screens, config loading.
- **M1 — Read-only browser**: policies tree + YAML view + action highlighting; Runs/Resources screens reading an existing `-s` output dir given on the CLI (`lazyc7n --output <dir>`). No process spawning yet. *(Already useful and zero-risk: first public release candidate.)*
- **M2 — Validate & dry-run**: runner abstraction, job streaming, per-run state dir, run history, log viewer.
- **M3 — Live run**: safety model §6 complete, preflight + typed confirmation, non-pull-mode handling.
- **M4 — Schema browser & polish**: schema cache, fuzzy search, `$EDITOR` integration, theming, docker/command backends, prune command.
- **M5 — Release**: `cargo-dist` (or equivalent) binaries for Linux/macOS/Windows, crates.io, Homebrew tap, AUR; demo GIF (vhs), docs site optional.

Post-1.0 ideas (not committed): diff between two runs of the same policy, export to CSV/JSON, reading from S3 output (`s3://`), Azure/GCP polish, a `lazyc7n run` headless mode, optional read-only push of run outputs to a cc-visualizer watch directory.

## 9. Open questions (decide during M0–M1)
1. Binary name: `lazyc7n` (current choice) vs `lazy-c7n`. Repo name stays `lazy-c7n`. Check crates.io / GitHub / package-manager collisions before publishing.
2. YAML crate choice (see §7) and whether to preserve comments/line numbers for "jump to line in `$EDITOR`".
3. Exact c7n flags/output file set per current c7n release **[verify]** — first task of M0: install c7n in a venv, run against the fixtures, and record real output in `tests/fixtures/real/`.
4. Policy discovery: only files passed/configured, or recursive `*.yml|*.yaml` under `policy_dirs` filtered by top-level `policies:` key? (Proposed: the latter.)
5. Windows support level for v0.x (subprocess groups/signals differ).

## 10. Licensing, liability and positioning

- **License**: dual `MIT OR Apache-2.0` (Rust ecosystem convention), both with their standard "AS IS / no warranty / no liability" clauses. Include both `LICENSE-MIT` and `LICENSE-APACHE` at release time (only MIT is committed in the scaffold; add Apache-2.0 text from the canonical source when scaffolding, do not retype it from memory).
- **README disclaimer (required wording, adapt as needed)**: lazy-c7n is an independent, community project. It is not affiliated with, endorsed by, or sponsored by the Cloud Custodian project or the CNCF. "Cloud Custodian" and "c7n" are used only to describe compatibility. You run policies with your own credentials at your own risk; live runs can modify or delete cloud resources; review the dry-run output first.
- No telemetry, no network calls of its own (only those made by the `custodian`/`docker` process the user asks for). State this in the README; it is a selling point.
- Contribution policy: DCO sign-off or plain PRs under the same license (decide at first external PR).

## 11. Testing strategy

- **Unit**: safety classification, policy parsing (lenient), output parsing, run-store retention.
- **State machine tests**: feed `Msg` sequences into `update()` and assert the live-run gate can't be bypassed (this is the most important test file in the repo).
- **Snapshot tests**: render each screen with `TestBackend` + `insta`.
- **Process tests**: a fake `custodian` (shell/Python script on `PATH`) that emits scripted stdout/stderr/exit codes and writes fixture output dirs → exercises runner, streaming, cancellation without any cloud access.
- **Fixtures**: synthetic c7n output; `../cc-visualizer/test-data/generate_synthetic_data.py` generates a compatible layout (copy it into `tests/fixtures/tools/` when needed; it is deterministic with seed 42). Plus real captured output from M0 step 3 of §9.
- CI: `cargo fmt --check`, `cargo clippy -- -D warnings`, `cargo test` on Linux/macOS/Windows.
- Real-cloud tests are **never** part of CI; a manual checklist lives in `docs/manual-testing.md` (to be written).
