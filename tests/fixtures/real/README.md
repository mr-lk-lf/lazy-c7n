# Real `custodian` output

Captured by running the real `custodian` CLI against a **local AWS emulator**
(no cloud account, no cost), with `tests/fixtures/tools/capture-real.sh`.
Directory name: `c7n-<custodian version>-<emulator>`.

Each scenario directory holds:

| File | Content |
|---|---|
| `argv.txt` | the `custodian ...` command line (run from the capture's work dir) |
| `exit_code.txt` | process exit code |
| `stdout.txt` / `stderr.txt` | raw streams (c7n logs to **stderr**; stdout only carries `report`/`version`/`schema` output) |
| `out/` | the `-s` output dir exactly as c7n wrote it |

Scenarios: `validate-ok`, `validate-invalid`, `dryrun`, `dryrun-glob` (`-p 's3-*'`),
`dryrun-multiregion` (`-r` twice), `live` (actions executed against the emulator),
`live-periodic` (non-pull mode without `--dryrun`: provisions a Lambda; moto rejects the role
and exits 2, Floci completes it with the EventBridge rule and exits 0),
`dryrun-api-error` (unreachable endpoint), `report-json`.

Scrubbing: absolute paths are replaced with `<repo>`, `<work>` and `~`. Account id,
bucket owners and instance ids are the emulator's fake values. If you capture against
a real account, write to `tests/fixtures/real-unscrubbed/` (git-ignored) and scrub before
copying anything here.

Regenerate:

```sh
python3 -m venv .venv-emu && .venv-emu/bin/pip install 'moto[server]'
.venv-emu/bin/moto_server -p 5055 &
tests/fixtures/tools/capture-real.sh

# or against Floci (needs Docker)
docker run -d --name floci -p 4566:4566 floci/floci:latest
LC7N_EMU_URL=http://localhost:4566 LC7N_EMU_NAME=floci tests/fixtures/tools/capture-real.sh
```

Captures: `c7n-0.9.52-moto` (moto 5.2.3), `c7n-0.9.52-floci` (Floci 2.1.0). Both give the same
file layout and resource counts; they differ in `live-periodic` (above) and in the fake ids.
