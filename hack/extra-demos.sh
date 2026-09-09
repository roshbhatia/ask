#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"
if [[ ${1:-} == --check && $# == 1 ]]; then
  go run ./hack/docgen --check
  bash hack/screenshots.sh --check
  exit 0
fi
if [[ $# != 1 ]]; then
  printf 'usage: %s --check | provider\nChoose one authenticated provider to record.\n' "$0" >&2
  exit 2
fi
exec bash hack/demo-extra.sh "$1" --record
