# Releasing lazyc7n

## Cutting a release

1. `main` is green in CI (lint, tests on Linux/macOS/Windows, govulncheck).
2. Tag and push:
   ```sh
   git tag v0.1.0-alpha.1      # -alpha/-beta/-rc tags become GitHub pre-releases
   git push origin v0.1.0-alpha.1
   ```
3. `.github/workflows/release.yml` runs the tests and GoReleaser (`.goreleaser.yaml`), which publishes to GitHub Releases:
   - `lazyc7n_<version>_<os>_<arch>.tar.gz` (Linux, macOS) and `.zip` (Windows), amd64 and arm64, each with README, both licences and `THIRD_PARTY_LICENSES.md`;
   - `.deb`, `.rpm`, `.apk` packages;
   - `checksums.txt` (SHA-256; `install.sh` refuses archives that do not match).
4. Check the release page, then `curl -fsSL .../install.sh | sh` and `lazyc7n version` on a clean machine.

Try it locally first, publishing nothing: `goreleaser release --snapshot --clean` (output in `dist/`).

The repository must be **public** for `install.sh`, `go install` and anonymous downloads to work.

## Channels

| Channel | Status | What it takes |
|---|---|---|
| GitHub Releases (binaries, checksums) | ready | push a tag |
| `curl … install.sh \| sh` | ready | public repo + a release |
| `go install …/cmd/lazyc7n@latest` | ready | public repo + a tag |
| `.deb` / `.rpm` / `.apk` files | ready | attached to each release (`dpkg -i`, `rpm -i`) |
| Homebrew (own tap) | next | create repo `vstrofago/homebrew-tap`, a token with write access to it as secret `HOMEBREW_TAP_GITHUB_TOKEN`, and a `homebrew_casks` section in `.goreleaser.yaml`; then `brew install vstrofago/tap/lazyc7n` |
| Scoop / winget (Windows) | later | Scoop bucket repo (GoReleaser `scoops`); winget needs a PR to microsoft/winget-pkgs per release |
| AUR (Arch) | later | an AUR account and a `lazyc7n-bin` package (GoReleaser `aurs`) |
| apt / dnf repositories (`apt install lazyc7n`) | later | a hosted, signed package repository (e.g. Cloudsmith, packagecloud, or GitHub Pages with a GPG key); the .deb/.rpm files already exist |
| Homebrew core (`brew install lazyc7n` without a tap) | much later | Homebrew requires a notable, stable project (stars, releases) |

## Version

`lazyc7n version` prints the release version, commit and build date (set by GoReleaser through `-ldflags`), the Go version and platform, and the custodian version of the configured backend. Builds from source show the Go module version or the git commit.
