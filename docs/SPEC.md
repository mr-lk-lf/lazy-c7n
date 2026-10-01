# lazy-c7n — Specification (v0.1 draft)

A terminal UI (TUI) wrapper around the [Cloud Custodian](https://cloudcustodian.io/) CLI (`custodian`, a.k.a. c7n). Think `lazygit` / `lazydocker`, for c7n: browse policies, validate and dry-run them, run them, and inspect reports and logs without memorising flags.

Status legend used below: **[verify]** = written from memory/docs, must be confirmed against a real `custodian` install (`custodian run -h`, real output dirs) before being relied on. **[verified c7n 0.9.52]** = confirmed on 2026-09-30 against `custodian` 0.9.52 (Python 3.14) running against a local AWS emulator (moto server); raw captures in `tests/fixtures/real/c7n-0.9.52-moto/`.

## 1. Goals and non-goals

### Goals
- Make day-to-day c7n usage fast: list policies, validate, dry-run, run, read results and logs, all from one screen-driven tool.
- Be a thin, honest wrapper: every action maps to a visible `custodian ...` command line the user can see (and copy).
- **Safe by default**: dry-run is the default, real runs need explicit confirmation.
- Single static binary, no server, no database, no account, no telemetry.
- Fully open source, non-profit (MIT OR Apache-2.0, see §10).

### Non-goals
- Not a reimplementation of c7n. Policy evaluation, filters, actions and cloud API calls are always done by the real `custodian` executable.
- No hosting, no multi-user features, no RBAC, no central history.
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
| Validate | `custodian validate [--strict] <file>...` | exit 0 valid / 1 invalid; messages on stderr. Only the first schema error per file is reported |
| Dry-run | `custodian run --dryrun -s <out> [-p <policy>]... [-r <region>]... <file>` | `-d`/`--dryrun`/`--dry-run` |
| Run | `custodian run -s <out> ... <file>` | gated |
| Report | `custodian report -s <out> --format json [-p <policy>] <file>` | optional; JSON = `resources.json` items plus `policy`, `region`, `CustodianDate`, `c7n:MatchedFilters` |
| Schema | `custodian schema [--json] [<cloud>.<resource>[.<category>.<item>]]` | cache result per c7n version |
| Version | `custodian version` | bare version (`0.9.52`) on stdout; shown in status bar; cache key for schema. `version --debug` dumps Python/platform/pip freeze |

**[verified c7n 0.9.52]** CLI facts:
- Subcommands: `run, schema, report, logs, metrics, version, validate` (`logs`/`metrics` are hidden from `-h`).
- `run` flags: `-s/--output-dir` (required; dir or `s3://`), `-p/--policies` (repeatable, **glob** via fnmatch: `-p 's3-*'`), `-t/--resource` (repeatable), `-r/--region` (repeatable, `all` allowed), `--profile`, `--assume`, `--external-id`, `-f/--cache` (default `~/.cache/cloud-custodian.cache`), `--cache-period` (minutes, default 15), `--session-policy`, `-d/--dryrun`, `--skip-validation`, `-m/--metrics-enabled`, `--trace`, `-l/--log-group`, `-v`, `-q` (repeatable).
- All logging goes to **stderr**; `run` and `validate` write nothing to stdout. Log line format: `YYYY-MM-DD HH:MM:SS,mmm: <logger>:<LEVEL> <message>`, e.g. `custodian.policy:INFO policy:<name> resource:<type> region:<r> count:<n> time:<s>` and `policy:<name> action:<action-class> resources:<n> execution_time:<s>`.
- Exit codes of `run`: `0` all policies ok; `2` if any policy raised (the others still run; stderr ends with `The following policies had errors while executing` + the list of names, preceded by the Python traceback).
- Resource cache: c7n caches `describe` results for `--cache-period` minutes in the `-f` file, keyed by account/region/resource type. A live run shortly after a dry-run reuses the dry-run's resource list. lazy-c7n should pass `-f <state>/c7n.cache` (or expose it) so behaviour is explicit, and show the cache period in the preflight.
- `pip install c7n` ships **AWS only** (387 resource types in 0.9.52); Azure/GCP/etc. need `c7n_azure`, `c7n_gcp`, … installed in the same env. `schema --json` is ~3.4 MB with `definitions.resources["<cloud>.<type>"].{actions,filters,policy}` plus `definitions.policy-mode`.

### Invocation backends (configurable)
1. `binary` (default): `custodian` on `$PATH`, or an explicit path (e.g. a venv).
2. `docker` **[verified c7n 0.9.52, image `cloudcustodian/c7n:latest` of 2026-09-03, against Floci]**: image `cloudcustodian/c7n`, `ENTRYPOINT ["/usr/local/bin/custodian"]`, so arguments after the image name are custodian's (`docker run ... cloudcustodian/c7n run -s ... <file>`). Runs as non-root user `custodian` (uid 1001), `WORKDIR`/`VOLUME` `/home/custodian`, which is mode `0750`. Ships AWS, Azure, GCP, Kubernetes, OCI, Tencent Cloud, OpenStack and AWSCC providers (no `c7n-org`, no `c7n-mailer`). Findings that shape the backend:
   - With the image's default user, c7n **cannot write** to a bind-mounted output dir owned by the host user (`PermissionError`, exit 2). Run with `--user <host uid>:<host gid>`: output files are then owned by the host user.
   - With `--user` set, mounts under `/home/custodian` are unreadable (0750, owned by 1001). Mount elsewhere, e.g. policies read-only at `/lazyc7n/policies` and the run dir at `/lazyc7n/run`.
   - A uid that does not exist in the image gets `HOME=/`, and c7n's default cache (`~/.cache/...`) fails with `PermissionError: '/.cache'`. Always pass `-f /lazyc7n/run/c7n.cache` (we pass `-f` anyway, §3).
   - `metadata.json` then records container paths (`config.output_dir = /lazyc7n/run/out`); lazy-c7n must map them back, never open them as host paths.
   - Credentials: env passthrough (`-e AWS_...` / `--env-file`) as in the official quickstart; mounting `~/.aws` needs the same uid care. Verified with env vars only.
   - Dev against an emulator on the host (Linux): `--add-host=host.docker.internal:host-gateway -e AWS_ENDPOINT_URL=http://host.docker.internal:4566`.
   - Env passthrough is by prefix (`AWS_`, `AZURE_`, `GOOGLE_`, `CLOUDSDK_`, `ARM_`, `OCI_`, `TENCENTCLOUD_`, `OS_`, `KUBECONFIG`). Variables that hold host file paths (`AWS_CONFIG_FILE`, `AWS_CA_BUNDLE`, `GOOGLE_APPLICATION_CREDENTIALS`, …) are passed too but point to files that do not exist in the container unless mounted with `docker_args`.
   - Proven command: `docker run --rm --user $(id -u):$(id -g) -e AWS_... -v <policydir>:/lazyc7n/policies:ro -v <rundir>:/lazyc7n/run cloudcustodian/c7n run [--dryrun] -f /lazyc7n/run/c7n.cache -s /lazyc7n/run/out -p <policy> /lazyc7n/policies/<file>` (dry-run and live, exit 0, same output layout as the binary backend).
   - Windows/macOS Docker Desktop uid mapping is not verified.
3. `command`: arbitrary prefix (e.g. `uvx --from c7n custodian`, `aws-vault exec prod --`).

The backend is an abstraction (`trait Runner`) so a **fake runner** can be used in tests (§11).

### Report (styled resource view)
`custodian report -s <dir> --format csv -p <policy> <file>` prints c7n's default report fields for the resource type (e.g. `aws.ec2`: InstanceId, tag:Name, InstanceType, LaunchTime, VpcId, PrivateIpAddress; `aws.s3`: Name, CreationDate) **[verified c7n 0.9.52]**. It reads `<dir>/<policy>/resources.json` and works offline. `-s` is the directory that contains the policy dir (`<out>` or `<out>/<region>`). lazy-c7n writes the policy recorded in `metadata.json` to a temporary JSON policy file, so any output dir can be reported, even without its original policy file. Fixture: `tests/fixtures/real/*/report-csv/`.

### c7n output layout (what we read back)
With `-s <out>` c7n writes one sub-directory per policy **[verified c7n 0.9.52]**:

```
<out>/<policy-name>/            # single region
<out>/<region>/<policy-name>/   # when more than one -r is given (one subdir per region)
  metadata.json        # always written (at context exit, i.e. last), also when the policy errored
  resources.json       # matched resources, raw provider API shape; `[]` when nothing matched.
                       #   ABSENT when the policy raised before/while fetching, and for live runs of
                       #   non-pull modes (provisioning does not evaluate resources)
  custodian-run.log    # per-policy log (same lines as stderr for that policy, incl. tracebacks)
  action-<action-class-name>   # optional, no extension, JSON; only when an action returns results
                               #   (e.g. `tag`/`mark-for-op` return nothing, so no file)
```

`metadata.json` top-level keys: `policy` (the policy spec as loaded), `version` (c7n version), `execution` = `{id, start, end_time, duration}` (`start`/`end_time` are **epoch seconds as floats**; there is **no `execution.end`**), `config` (the effective CLI options: `region`, `regions`, `account_id`, `profile`, `dryrun`, `output_dir`, `cache`, `cache_period`, `assume_role`, `external_id`, `policy_filters`, …; no credentials, but treat as potentially sensitive), `sys-stats`, `api-stats` (map `"service.Operation": count`), `metrics` (list of `{MetricName, Timestamp, Value, Unit}`).

Important: c7n reuses `<out>/<policy-name>/` — running the same policy again into the same `-s` dir **overwrites** it. So lazy-c7n gives **every run its own output dir** (§5) instead of reusing one. This is what makes a local run history possible.

## 4. Screens and navigation

Layout: lazygit-style panes. Left column = lists, right = detail/preview, bottom = key hints, top/bottom status bar (custodian version, backend, active profile/region *as read from env*, mode badge DRY/LIVE).

| Screen | Left | Right |
|---|---|---|
| **Policies** | tree: policy dir → files → policies, fuzzy filter `/` | YAML (syntax-highlighted), summary: resource, mode, filters, **actions (mutating ones highlighted)** |
| **Runs** | history (newest first): time, dry/live, status, policies and matches, ⚠ when destructive actions matched, label | run summary (matches per resource type and region; how many matches destructive / changing actions would hit (dry-run) or hit (live), per action; failed policies; deployed Lambdas), policy table with coloured ACTIONS, log tab (`t`), command line. Counts are matches, not distinct resources (two policies can match the same resource) |
| **Resources** | (full width, top) table of the matched resources with the columns c7n itself chooses for the resource type (`custodian report --format csv`) | (full width, bottom) card of the selected resource: report fields, tags, `c7n:MatchedFilters`; `t` switches to the raw JSON |
| **Schema** | resource types → filters/actions | help text and JSON schema from `custodian schema` |
| **Jobs** | running/queued jobs | live streaming stdout/stderr |

Keys as implemented (M1–M4): `Tab`/`Shift+Tab` or `1`–`5` switch screen; `h`/`l` (or arrows) switch pane; `j`/`k`, `g`/`G`, `PgUp`/`PgDn` move or scroll; `Enter` open; `Esc` back (clears the filter first, then the selection on Policies); `/` fuzzy filter of the left list; `?` all keys; `q` quit (asks again while jobs run; `Ctrl+C` always quits, interrupting running jobs).
Policies: `Space` select (on a file row: all its policies), `v` validate, `d` dry-run, `R` live run (gated, Policies screen only), `e` open in `$EDITOR` at the policy's line, `y` copy the dry-run command, `r` reload. Actions apply to the selected policies, or else to the policy (or file) under the cursor.
Runs: `Enter` policy table → resources, `t` toggle the per-policy log, `y` copy the command, `r` reload. Resources: `t` card/JSON, `y` copy the resource id. Anywhere: `T` next colour theme (for trying them; the config keeps the choice). Jobs: `x` cancel (interrupt, kill after 5 s). Schema: `Enter` browse / load help, `r` re-run `custodian schema --json`.

## 5. Data model and state

No database. Plain files only.

- **Config** (`$XDG_CONFIG_HOME/lazyc7n/config.toml`, project-local `.lazyc7n.toml` overrides):
  ```toml
  policy_dirs = ["./policies"]  # replaced by policy paths given as arguments
  state_dir = ""             # empty = $XDG_STATE_HOME/lazyc7n
  keep_runs = 200            # run history retention (0 = keep all)
  theme = "lazyc7n"          # lazyc7n | terminal | catppuccin | gruvbox | everforest | tokyonight | dracula
  appearance = "auto"        # auto (follow the terminal background) | dark | light
  [runner]
  kind = "binary"            # binary | docker | command
  custodian = "custodian"    # or path / venv
  # command = ["uvx", "--from", "c7n", "custodian"]   # required for kind = "command"
  docker = "docker"          # or podman
  image = "cloudcustodian/c7n"
  docker_args = []           # e.g. ["--network", "host"] to reach an emulator on the host
  [defaults]
  region = ""                # empty = let custodian decide
  cache_period = ""          # passthrough, optional
  [safety]
  default_dry_run = true     # changing this to false is allowed but shows a warning
  confirm_live = "type-name" # only value in v0; "yes-no" is rejected at startup (PM decision 2026-09-30)
  ```
- **State dir** (`$XDG_STATE_HOME/lazyc7n/`):
  ```
  runs/<run-id>/           # run-id = <UTC timestamp>-<short id>
    run.json               # our own record: argv, backend, dry_run, policies, started/ended, exit code, custodian version
    out/                   # passed as `-s` → c7n's native output (<policy>/metadata.json, resources.json, ...)
    stdout.log stderr.log  # raw captured streams
  schema-cache/<c7n-version>.json
  ```
- Retention: after each run the oldest finished runs beyond `keep_runs` are deleted; `lazyc7n -prune [-prune-days N]` does it by hand (and by age). Unfinished runs are never deleted.
- The c7n resource cache is `<state>/c7n.cache` (always passed as `-f`).
- In-memory model: Elm-style architecture (see §7): `App { screen, policies, runs, jobs, config, ... }`; all I/O is performed by tasks that send `Msg`s back.

## 6. Safety model (first-class feature)

1. **Dry-run by default.** The `d` key is dry-run; live run is a different key (`R`, shift) and a different badge colour.
2. **Pre-flight summary before any live run**: exact command line, policy names, region(s), backend, and the **actions** each policy will perform.
3. **Action classification** (static, by c7n action `type`; `internal/c7n/safety.go`, lists built from the 170 AWS action names of c7n 0.9.52):
   - *destructive*: `terminate`, `delete`, `delete-*`, `stop`, `detach`, `release`, `deregister`, `disable`, `disassociate`, `suspend`, `pause`, `reboot`, `cancel`, `revoke-access`, `schedule-deletion`, `trim-versions`, `remove-*` (except `remove-tag`)
   - *notify-only*: `notify`, `post-finding`, `post-item`, `put-metric`, `webhook`, `no-op`
   - *mutating*: everything else (`tag`, `mark-for-op`, `modify-*`, `set-*`, `invoke-lambda`, …). Unknown action types are **mutating** (fail closed).
   - Overriding these lists from the config: not in v0 (planned for M3).
4. **Confirmation gate** for live runs (`internal/app/gate.go`, tests in `gate_test.go`; PM decisions 2026-09-30):
   - `R` opens a full-body red dialog listing each policy (name, resource, actions, `[destructive]`, `[mode X: deploys Lambda]`), the exact command line (once the runner exists), the backend, and a red `!! N policies have DESTRUCTIVE actions: …` line when relevant.
   - The user types, exactly (case-sensitive, surrounding spaces ignored): the **policy name** for one policy, the **number of policies** (e.g. `3`) for several. `ALL` was rejected: typing the number forces looking at the list.
   - A wrong answer runs nothing, clears the input and shows "does not match"; Esc closes the dialog at any step; ctrl+c quits the app. While the dialog is open every other key is typed text (no tab switching, `q` does not quit, a second `R` is a letter). Pasting is allowed, but a pasted newline never acts as Enter.
   - The dialog confirms a frozen copy of the request: changing the selection behind it cannot change what runs.
   - Only one mode, `type-name`: `confirm_live = "yes-no"` is rejected at startup in v0 (fail closed). The gate itself never reads the config.
5. **Non-`pull` modes warning [verified c7n 0.9.52]**: every mode other than `pull` (`periodic`, `schedule`, `phd`, `cloudtrail`, `ec2-instance-state`, `asg-instance-state`, `guard-duty`, `config-poll-rule`, `config-rule`, `hub-finding`, `hub-action`) is a serverless mode. Source (`c7n/policy.py`, `Policy.__call__`): with `--dryrun` **any** mode is evaluated once as `pull` (runtime-only filters trimmed, actions skipped), so dry-run is safe for them. **Without** `--dryrun` a serverless mode calls `provision()`: it creates/updates the Lambda function and its event source (CloudWatch Events rule, Config rule, …) and does **not** evaluate resources (no `resources.json`). Confirmed on the emulators: the live run of a `periodic` policy called `lambda:CreateFunction` (moto then rejects the role, exit 2); on Floci it completes and also creates the EventBridge rule and target (`events:PutRule`, `events:PutTargets`, `lambda:AddPermission`), exit 0, still no `resources.json`. lazy-c7n flags these policies explicitly and, after the normal confirmation, shows a second step ("DEPLOYS INFRASTRUCTURE · step 2/2": what gets created, the non-pull policies and their mode) where the user must type `DEPLOY`. Unknown modes count as non-pull (fail closed). Mode list source of truth: `custodian schema mode`.
6. **Credentials never shown**: only non-secret context (profile name, region, account id *if already present in c7n output*). Env var values matching `*KEY*|*SECRET*|*TOKEN*` are never rendered or logged.
7. **Audit trail**: every run, dry or live, has a `run.json` with the full argv (secrets redacted).
8. **No implicit live**: there is no CLI flag that skips the gate in interactive mode. (A future non-interactive `lazyc7n run --yes` for scripts, if ever added, must be explicit and documented as a scripting escape hatch.)

## 7. Architecture (Go + Bubble Tea)

**Stack decision (2026-09-30, replaces the initial Rust/ratatui choice).** The maintainer is a PM/tester who may one day need to maintain the code by hand, and wants a rich, pleasant UI and stability. Go + the Charm stack wins on all three: Go is quick to learn and reads plainly; Charm's components and styling give a polished UI with little code; Bubble Tea/Lip Gloss/Bubbles are on stable semver v2 (ratatui is still 0.x) and Go keeps its compatibility promise. What Rust gave us (compile-time exhaustive matching for the safety gate) is replaced by the `exhaustive` linter in CI plus mandatory state-machine tests.

Stack (add/upgrade dependencies with `go get <module>@latest`; never type versions from memory):
- TUI: `charm.land/bubbletea/v2` (Elm architecture: `Init`/`Update`/`View`), `charm.land/lipgloss/v2` (styling, adaptive light/dark theme), `charm.land/bubbles/v2` (list, table, viewport, textinput, spinner, help/key bindings). Candidates for later: Huh (forms) for the live-run confirmation and Glamour (markdown rendering) for schema help; check their current module path and major version when adding them.
- Config: `github.com/BurntSushi/toml`; XDG paths: `github.com/adrg/xdg` (config and, later, state dir).
- JSON: stdlib `encoding/json` (decode into lenient structs / `map[string]any`; unknown fields are ignored by default).
- YAML: open question §9.2 (`go.yaml.in/yaml/v3`, the maintained successor of the archived `gopkg.in/yaml.v3`, vs `github.com/goccy/go-yaml`, which keeps positions/comments for "jump to line"). Only needed for *display/summary*; never fail on unknown c7n fields.
- Syntax highlighting for YAML/JSON: `github.com/alecthomas/chroma/v2` (what Glamour uses) or a small hand-rolled highlighter; decide after a spike.
- Subprocesses: stdlib `os/exec` + goroutines; each job's lines are sent into the program as messages (`Program.Send` or a `tea.Cmd` that reads a channel).
- CLI flags: stdlib `flag` for now; move to a subcommand library only if headless subcommands are ever added.
- Testing: stdlib `testing`; views are tested by calling `View()` and comparing the ANSI-stripped text (`github.com/charmbracelet/x/ansi`), golden files when screens stabilise; a fake `custodian` script for process tests.
- Lint: `gofmt`, `go vet`, `golangci-lint` v2 with `exhaustive` enabled (`.golangci.yml`).
- Releases: GoReleaser (§8 M5).

Package layout:
```
cmd/lazyc7n/         # main: flags, config loading, tea.NewProgram(...).Run()
internal/
  app/               # Model (state), Update (pure), View; messages and commands; one file per screen as they grow
  ui/                # theme (Lip Gloss styles, light/dark), shared components
  config/            # config loading + validation
  runner/            # Runner interface; binary.go, docker.go, command.go, fake.go
  c7n/
    policy.go        # lenient policy YAML model (name, resource, mode, filters, actions)
    output.go        # read metadata.json / resources.json / custodian-run.log
    schema.go        # `custodian schema --json` cache + lookup
    safety.go        # action classification + preflight summary
  store/             # runs dir management (run.json, retention)
```

Key design rules:
- `Update()` is pure (no I/O); side effects are `tea.Cmd`s executed by the Bubble Tea runtime. This makes the safety gate testable as a state machine: feed messages into `Update()`, assert on the returned model and commands.
- Subprocesses: stream stdout/stderr line-by-line as `jobOutputMsg` messages; support cancel (SIGINT first, then kill after timeout). Run in their own process group.
- Parsing must be **lenient**: c7n versions/providers differ; unknown fields ignored, missing optional fields shown as `-`.
- Large `resources.json` (100k+ items): load lazily/paged, never block the render loop.
- Multi-cloud: v0.1 targets AWS shapes first, but nothing in the core may assume AWS (Azure/GCP resource ids differ). Resource-id extraction is a pluggable table.

## 8. Milestones

- **M0 — Scaffold**: Go module, CI (gofmt, go vet, golangci-lint, tests on 3 OSes), terminal setup/teardown (Bubble Tea restores the terminal on exit and panic), empty screens, config loading.
- **M1 — Read-only browser**: policies tree + YAML view + action highlighting; Runs/Resources screens reading an existing `-s` output dir given on the CLI (`lazyc7n --output <dir>`). No process spawning yet. *(Already useful and zero-risk: first public release candidate.)*
- **M2 — Validate & dry-run**: runner abstraction, job streaming, per-run state dir, run history, log viewer.
- **M3 — Live run**: safety model §6 complete, preflight + typed confirmation, non-pull-mode handling.
- **M4 — Schema browser & polish**: schema cache, fuzzy search, `$EDITOR` integration, theming, docker/command backends, prune command.
- Status (2026-09-30): **M0–M4 implemented** on branch `tui-m0-m4`, tested end-to-end against Floci with the binary and docker backends. Not released.
- **M5 — Release**: GoReleaser binaries for Linux/macOS/Windows (amd64/arm64), `go install github.com/vstrofago/lazy-c7n/cmd/lazyc7n@latest`, Homebrew tap, AUR, Scoop/winget; demo GIF (vhs), docs site optional.

Post-1.0 ideas (not committed): diff between two runs of the same policy, export to CSV/JSON, reading from S3 output (`s3://`), Azure/GCP polish, a `lazyc7n run` headless mode.

## 9. Open questions (decide during M0–M1)
1. Binary name: `lazyc7n` (current choice) vs `lazy-c7n`. Repo name stays `lazy-c7n`. Check GitHub / Homebrew / AUR / package-manager collisions before publishing.
2. ~~YAML library~~ — decided (M1): `go.yaml.in/yaml/v3`, parsed as `yaml.Node` so every policy keeps its line (used by `e`, the YAML view and the gate). Highlighting is a small hand-rolled line highlighter (no chroma).
3. ~~Exact c7n flags/output file set~~ — resolved for 0.9.52 (§3, `tests/fixtures/real/c7n-0.9.52-moto/`). Re-run `tests/fixtures/tools/capture-real.sh` when bumping the supported c7n version.
4. ~~Policy discovery~~ — decided (M1): recursive `*.yml|*.yaml` under each path of `policy_dirs` (or the paths given as arguments; a path may be a file), keeping files with a top-level `policies:` key; hidden dirs and `node_modules` skipped; broken files that look like policy files are listed with their error.
5. Windows support level for v0.x (subprocess groups/signals differ).

## 10. Licensing, liability and positioning

- **License**: dual `MIT OR Apache-2.0`, both with their standard "AS IS / no warranty / no liability" clauses. Include both `LICENSE-MIT` and `LICENSE-APACHE` at release time (only MIT is committed in the scaffold; add Apache-2.0 text from the canonical source when scaffolding, do not retype it from memory).
- **README disclaimer (required wording, adapt as needed)**: lazy-c7n is an independent, community project. It is not affiliated with, endorsed by, or sponsored by the Cloud Custodian project or the CNCF. "Cloud Custodian" and "c7n" are used only to describe compatibility. You run policies with your own credentials at your own risk; live runs can modify or delete cloud resources; review the dry-run output first.
- No telemetry, no network calls of its own (only those made by the `custodian`/`docker` process the user asks for). State this in the README; it is a selling point.
- Contribution policy: DCO sign-off or plain PRs under the same license (decide at first external PR).

## 11. Testing strategy

- **Unit**: safety classification, policy parsing (lenient), output parsing, run-store retention.
- **State machine tests**: feed `Msg` sequences into `update()` and assert the live-run gate can't be bypassed (this is the most important test file in the repo).
- **Snapshot tests**: render each screen's `View()` at a fixed size, strip ANSI, compare with golden files.
- **Process tests**: a fake `custodian` (shell/Python script on `PATH`) that emits scripted stdout/stderr/exit codes and writes fixture output dirs → exercises runner, streaming, cancellation without any cloud access.
- **Fixtures**: real captured output in `tests/fixtures/real/` (see its README), plus synthetic output (large/edge cases) from a small deterministic generator in `tests/fixtures/tools/` (to be written, when needed).
- **Local cloud emulator** for manual/dev testing without a cloud account: `moto` server (`pip install 'moto[server]'` in `.venv-emu`, no Docker needed) or Floci (`floci/floci` image, LocalStack-compatible, port 4566; needs Docker; **verified with c7n 0.9.52 on 2026-09-30**, captures in `tests/fixtures/real/c7n-0.9.52-floci/`: same output layout and resource counts as moto, and unlike moto it completes a live non-pull provisioning: `lambda:CreateFunction`, `lambda:AddPermission`, `events:PutRule`, `events:PutTargets`). Point c7n at it with `AWS_ENDPOINT_URL` + fake credentials and `AWS_CONFIG_FILE=/dev/null` (see `tests/fixtures/tools/capture-real.sh`). Emulators are never a CI dependency.
- CI: `gofmt -l`, `go mod tidy -diff`, `go vet`, `golangci-lint run`, `go test ./...` (with `-race` on Linux/macOS) on Linux/macOS/Windows.
- Real-cloud tests are **never** part of CI; a manual checklist lives in `docs/manual-testing.md` (to be written).
