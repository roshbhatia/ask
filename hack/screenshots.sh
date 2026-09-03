#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_dir"
mkdir -p "$repo_dir/docs"
build_dir=$(mktemp -d)
trap 'rm -rf "$build_dir"' EXIT

go build -o "$build_dir/ask" .
install -m 0755 "$repo_dir/hack/fixtures/provider" "$build_dir/ask-provider-demo"
install -m 0755 "$repo_dir/hack/fixtures/demo" "$build_dir/ask-demo"
mkdir -p "$build_dir/providers/demo"
cp "$repo_dir/hack/fixtures/provider.yaml" "$build_dir/providers/demo/provider.yaml"
export ASK_PROVIDER_PATH="$build_dir/providers"

PATH="$build_dir:$PATH" freeze \
  --execute "ask-demo" \
  --output "$repo_dir/docs/ask.png" \
  --width 1100 \
  --padding 24 \
  --margin 16 \
  --window

PATH="$build_dir:$PATH" vhs hack/ask.tape --output "$repo_dir/docs/ask.gif"
