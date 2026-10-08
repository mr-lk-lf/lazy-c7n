<div align="center">

# lazy-c7n

**Cloud Custodian, without memorising a single flag.**

A fast, keyboard-driven terminal UI for [`custodian`](https://cloudcustodian.io/): browse your policies, validate, dry-run, run, and dig through results and logs, all from one screen.

*In the spirit of `lazygit` and `lazydocker`.*

[![CI](https://github.com/mr-lk-lf/lazy-c7n/actions/workflows/ci.yml/badge.svg)](https://github.com/mr-lk-lf/lazy-c7n/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/license-MIT%20OR%20Apache--2.0-blue)
![Status](https://img.shields.io/badge/status-early%20alpha-orange)

![lazy-c7n Policies screen](docs/screenshots/policies.png)

</div>

## Why you'll like it

You already write c7n YAML. But the loop around it is a pile of long commands: `custodian validate`, `custodian run --dryrun -s ./out -p 's3-*' -r us-east-1 ...`, then `cat out/*/resources.json | jq ...`. lazy-c7n turns that loop into something you can drive with a few keys.

- **Everything in one place.** Policies, runs, matched resources, logs and the c7n schema, one tab each.
- **Safe by default.** Dry-run is the default. Live runs sit behind a typed confirmation that can't be skipped by accident, with extra warnings for destructive actions.
- **No magic.** It is a thin wrapper: the real `custodian` does all the work and lazy-c7n shows you the exact command it runs.
- **Local and private.** One binary, no server, no database, no telemetry. Credentials come from your environment and are never stored or displayed.
- **Lenient parsing.** Unknown fields in policies and output are ignored, so it keeps working as c7n evolves.

## Where it is today

lazy-c7n is **early alpha**. Be honest with yourself before you star it: the foundation is built and tested, but the features that make it shine are being added milestone by milestone.

| Milestone | What | State |
|---|---|---|
| M0 | Scaffold: config loading, tabs, key help, clean terminal setup/teardown, CI on Linux/macOS/Windows | Done |
| M1 | Read-only browser: policy tree, YAML view, runs and resources from an existing output dir | Next |
| M2 | Validate and dry-run with live log streaming and run history | Planned |
| M3 | Live runs with preflight and typed confirmation | Planned |
| M4 | Schema browser, fuzzy search, `$EDITOR` integration, themes | Planned |
| M5 | Prebuilt binaries, Homebrew, AUR, Scoop | Planned |

Full design in [`docs/SPEC.md`](docs/SPEC.md).

## Try it

It takes about a minute (details in [Installation](#installation)):

```sh
git clone https://github.com/mr-lk-lf/lazy-c7n.git
cd lazy-c7n && go build -o lazyc7n ./cmd/lazyc7n && ./lazyc7n
```

Hop between screens with `Tab` / `Shift+Tab`, press `?` for keys, `q` to quit.

![Key help](docs/screenshots/help.png)

## Help shape it

This is the best moment to influence the project. Try it, then:

- Open an issue with the workflow you wish a c7n TUI made easier.
- Tell us what breaks on your OS, terminal or c7n version.
- Star the repo if you want to see it grow.

Contributions are welcome. Read [`docs/SPEC.md`](docs/SPEC.md) first; dev notes live in [`CLAUDE.md`](CLAUDE.md).

## Installation

There are no prebuilt releases yet, so build from source.

### Requirements

- [Go](https://go.dev/dl/) 1.26 or newer.
- [Cloud Custodian](https://cloudcustodian.io/docs/quickstart/index.html) (`custodian` on your `PATH`), for example:

  ```sh
  python3 -m venv .venv && .venv/bin/pip install c7n   # or: pipx install c7n
  ```

  Cloud credentials come from your environment as usual (AWS profile, env vars, ...). lazy-c7n never stores them.

### Build and run

```sh
git clone https://github.com/mr-lk-lf/lazy-c7n.git
cd lazy-c7n
go build -o lazyc7n ./cmd/lazyc7n
./lazyc7n
```

Optionally move the binary onto your `PATH`, e.g. `sudo mv lazyc7n /usr/local/bin/`.

Useful flags: `-config <file>` to use a specific config file, `-version` to print the version.

### Configuration (optional)

Config lives in `$XDG_CONFIG_HOME/lazyc7n/config.toml` (usually `~/.config/lazyc7n/config.toml`); a `.lazyc7n.toml` in the current directory overrides it. Example:

```toml
policy_dirs = ["./policies"]

[runner]
kind = "binary"          # binary | docker | command
custodian = "custodian"  # or a path, e.g. ".venv/bin/custodian"

[safety]
default_dry_run = true
confirm_live = "type-name"
```

See [`docs/SPEC.md`](docs/SPEC.md) for all options.

## Principles

- **Thin wrapper.** The real `custodian` executable does all the work; lazy-c7n shows you the exact command it runs.
- **Safe by default.** Dry-run first; live runs require explicit, typed confirmation and show which actions will execute.
- **Local only.** Single binary, no server, no database, no telemetry. Credentials come from your environment and are never stored or displayed.
- **Open source, non-profit.** MIT OR Apache-2.0, at your option.

## Disclaimer

lazy-c7n is an independent, community project. It is **not** affiliated with, endorsed by, or sponsored by the Cloud Custodian project or the CNCF. "Cloud Custodian" and "c7n" are used only to describe compatibility.

You run policies with your own credentials and at your own risk. Live runs can modify or delete cloud resources. Review dry-run output first. The software is provided "as is", without warranty of any kind (see [LICENSE-MIT](LICENSE-MIT) and [LICENSE-APACHE](LICENSE-APACHE)).
