# lazy-c7n

A terminal UI for the [Cloud Custodian](https://cloudcustodian.io/) CLI, in the spirit of `lazygit` and `lazydocker`: browse your policies, validate them, dry-run them, run them, and read the results and logs, without memorising flags.

See [`docs/SPEC.md`](docs/SPEC.md) for the design and roadmap.

## Screenshots

Early scaffold (M0): the screen frame, tabs and key help are in place; the screens themselves are still empty.

![Policies screen](docs/screenshots/policies.png)

![Key help](docs/screenshots/help.png)

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
