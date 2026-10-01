#!/usr/bin/env bash
# Write THIRD_PARTY_LICENSES.md: the licence (and NOTICE, if any) of every
# module compiled into lazyc7n. Shipped in release archives (GoReleaser
# runs this before building).
set -euo pipefail
cd "$(dirname "$0")/.."

out="${1:-THIRD_PARTY_LICENSES.md}"
{
  echo "# Third-party licences"
  echo
  echo "lazyc7n is built with the Go modules below. Their licences follow."
  for mod in $(go list -deps -f '{{if .Module}}{{.Module.Path}}{{end}}' ./cmd/lazyc7n | sort -u | grep -v '^github.com/vstrofago/'); do
    dir=$(go list -m -f '{{.Dir}}' "$mod")
    ver=$(go list -m -f '{{.Version}}' "$mod")
    echo
    echo "## $mod $ver"
    for f in "$dir"/LICENSE* "$dir"/LICENCE* "$dir"/COPYING* "$dir"/NOTICE*; do
      [ -f "$f" ] || continue
      echo
      echo '```'
      cat "$f"
      echo '```'
    done
  done
} >"$out"
echo "wrote $out"
