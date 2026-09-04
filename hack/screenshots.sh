#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_dir"

media_fingerprint() {
  {
    printf '%s\n' flake.lock flake.nix go.mod go.sum hack/ask.tape hack/screenshots.sh
    find cmd internal extras hack/fixtures -type f ! -name '*_test.go' -print | LC_ALL=C sort
  } | while IFS= read -r source_file; do
    sha256sum "$source_file"
  done | sha256sum | cut -d ' ' -f 1
}

fingerprint=$(media_fingerprint)
if [[ ${1:-} == "--check" ]]; then
  stored=$(<"$repo_dir/docs/ask.media.sha256")
  [[ -s $repo_dir/docs/ask.png ]]
  [[ -s $repo_dir/docs/ask.gif ]]
  if [[ $stored != "$fingerprint" ]]; then
    printf 'Ask media is stale; run hack/screenshots.sh\n' >&2
    exit 1
  fi
  exit 0
fi
if [[ $# -gt 0 ]]; then
  printf 'usage: %s [--check]\n' "$0" >&2
  exit 2
fi

build_dir=$(mktemp -d)
trap 'rm -rf "$build_dir"' EXIT
full=$(nix build "$repo_dir#full" --no-link --print-out-paths)

install -m 0755 "$repo_dir/hack/fixtures/provider" "$build_dir/ask-provider-demo"
mkdir -p "$build_dir/providers/demo"
cp "$repo_dir/hack/fixtures/provider.yaml" "$build_dir/providers/demo/provider.yaml"
mkdir -p "$build_dir/home" "$build_dir/config" "$build_dir/data" "$build_dir/state" "$build_dir/cache" "$build_dir/runtime"
export HOME="$build_dir/home"
export XDG_CONFIG_HOME="$build_dir/config"
export XDG_DATA_HOME="$build_dir/data"
export XDG_DATA_DIRS="$full/share"
export XDG_STATE_HOME="$build_dir/state"
export XDG_CACHE_HOME="$build_dir/cache"
export XDG_RUNTIME_DIR="$build_dir/runtime"
export ASK_PROVIDER_PATH="$build_dir/providers"
unset ASK_CONFIG ASK_PROVIDER ASK_PROVIDER_DEFAULT ASK_PROVIDERS_DIRECTORY

PATH="$build_dir:$full/bin:$PATH" freeze \
  --execute "ask -q -p demo \"Review the token parser change and summarize the evidence.\"" \
  --output "$build_dir/ask.png" \
  --width 1100 \
  --padding 24 \
  --margin 16 \
  --window

PATH="$build_dir:$full/bin:$PATH" vhs hack/ask.tape --output "$build_dir/ask.gif"
install -m 0644 "$build_dir/ask.png" "$repo_dir/docs/ask.png"
install -m 0644 "$build_dir/ask.gif" "$repo_dir/docs/ask.gif"
printf '%s\n' "$fingerprint" >"$repo_dir/docs/ask.media.sha256"
