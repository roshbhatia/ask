#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
mkdir -p "$repo_dir/docs"
build_dir=$(mktemp -d)
trap 'rm -rf "$build_dir"' EXIT

go build -o "$build_dir/ask" .
PATH="$build_dir:$PATH" freeze \
  --execute "ask --help" \
  --output "$repo_dir/docs/ask.png" \
  --width 1100 \
  --padding 24 \
  --margin 16 \
  --window
