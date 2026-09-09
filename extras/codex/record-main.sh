#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_dir"
recorded_release=github:roshbhatia/ask/v0.7.1
revision=6147beb23c88864180be2cccdec9a52dd1a3a6fc

fingerprint() {
  sha256sum hack/ask.tape hack/screenshots.sh extras/codex/record-main.sh docs/ask.gif docs/ask.png
}

if [[ ${1:-} == --check ]]; then
  [[ -s docs/ask.gif && -s docs/ask.png ]]
  [[ $(fingerprint) == "$(cat docs/ask.media.sha256)" ]] || {
    printf 'Ask recording changed; run hack/screenshots.sh with an authenticated Codex CLI\n' >&2
    exit 1
  }
  exit 0
fi
if [[ $# -gt 0 ]]; then
  printf 'usage: %s [--check]\n' "$0" >&2
  exit 2
fi

codex login status
build_dir=$(mktemp -d)
trap 'rm -rf "${build_dir:?}"' EXIT
core=$(nix build "$recorded_release#ask" --no-link --print-out-paths)
provider=$(nix build "$recorded_release#provider-codex" --no-link --print-out-paths)
mkdir -p "$build_dir/data" "$build_dir/providers/codex"
cp "$provider/share/ask/providers/codex/provider.yaml" "$build_dir/providers/codex/provider.yaml"
printf 'version: ask.config/v1\n' > "$build_dir/config.yaml"
export ASK_CONFIG="$build_dir/config.yaml"
export ASK_PROVIDERS_DIRECTORY="$build_dir/providers"
export XDG_DATA_HOME="$build_dir/data"
export XDG_DATA_DIRS="$build_dir/data"
export PATH="$core/bin:$provider/bin:$PATH"
unset ASK_PROVIDER ASK_PROVIDER_PATH ASK_PROVIDER_DEFAULT
unset GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_COMMON_DIR GIT_DIR GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_WORK_TREE

git clone --quiet https://github.com/roshbhatia/changes.git "$build_dir/changes"
git -C "$build_dir/changes" checkout --quiet --detach "$revision"
cd "$build_dir/changes"
vhs "$repo_dir/hack/ask.tape" --output "$build_dir/ask.gif"
git diff --exit-code --quiet
duration=$(ffprobe -v error -show_entries format=duration -of csv=p=0 "$build_dir/ask.gif")
frame_time=$(awk -v duration="$duration" 'BEGIN { print duration - 10 }')
ffmpeg -v error -y -i "$build_dir/ask.gif" -ss "$frame_time" -frames:v 1 "$build_dir/ask.png"
install -m 0644 "$build_dir/ask.png" "$repo_dir/docs/ask.png"
install -m 0644 "$build_dir/ask.gif" "$repo_dir/docs/ask.gif"
cd "$repo_dir"
fingerprint > docs/ask.media.sha256
