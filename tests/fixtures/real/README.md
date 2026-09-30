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
`live-periodic` (non-pull mode without `--dryrun`: tries to provision a Lambda and fails),
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
```
