# lazy-c7n

A terminal UI for the [Cloud Custodian](https://cloudcustodian.io/) CLI, in the spirit of `lazygit` and `lazydocker`: browse your policies, validate them, dry-run them, run them, and read the results and logs, without memorising flags.

> **Status: pre-alpha, not released.** Browsing, validate, dry-run, live runs (behind a typed confirmation), run history and the schema browser work; tested against local AWS emulators only. See [`docs/SPEC.md`](docs/SPEC.md).

## Try it (no cloud account needed)

```sh
python3 -m venv .venv && .venv/bin/pip install c7n                 # custodian
python3 -m venv .venv-emu && .venv-emu/bin/pip install 'moto[server]' # boto3 for seeding
scripts/floci-dev.sh            # starts Floci (Docker), seeds it, opens lazyc7n on examples/policies
scripts/floci-dev.sh --docker   # same, running custodian in the cloudcustodian/c7n image
```

## Usage

```sh
go build ./cmd/lazyc7n
./lazyc7n [policy files or dirs...]      # default: policy_dirs from the config (./policies)
./lazyc7n -output path/to/custodian-out  # also browse an existing `custodian run -s` dir (repeatable)
./lazyc7n -prune                         # trim the run history and exit
```

Screens: `1` Policies · `2` Runs · `3` Resources · `4` Schema · `5` Jobs. On Policies: `space` select, `v` validate, `d` dry-run, `R` live run, `e` edit, `/` filter. `?` shows every key.

Configuration: `$XDG_CONFIG_HOME/lazyc7n/config.toml`, overridden by `./.lazyc7n.toml` (see [`docs/SPEC.md` §5](docs/SPEC.md)).

## Principles

- **Thin wrapper.** The real `custodian` executable does all the work; lazy-c7n shows you the exact command it runs.
- **Safe by default.** Dry-run first; live runs require explicit, typed confirmation and show which actions will execute.
- **Local only.** Single binary, no server, no database, no telemetry. Credentials come from your environment and are never stored or displayed.
- **Open source, non-profit.** MIT OR Apache-2.0, at your option.

## Disclaimer

lazy-c7n is an independent, community project. It is **not** affiliated with, endorsed by, or sponsored by the Cloud Custodian project or the CNCF. "Cloud Custodian" and "c7n" are used only to describe compatibility.

You run policies with your own credentials and at your own risk. Live runs can modify or delete cloud resources. Review dry-run output first. The software is provided "as is", without warranty of any kind (see [LICENSE-MIT](LICENSE-MIT) and [LICENSE-APACHE](LICENSE-APACHE)).
