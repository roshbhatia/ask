#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
name=${1:?usage: demo-extra.sh provider [--record]}
mode=${2:-}
[[ $# -le 2 && (-z $mode || $mode == --record) ]] || exit 2
[[ $name != *[!a-z-]* && -f $root/extras/$name/provider.yaml ]] || exit 2
work=$(mktemp -d)
trap 'rm -rf "${work:?}"' EXIT
core=$(nix build "$root#ask" --no-link --print-out-paths)
provider=$(nix build "$root/extras#provider-$name" --no-link --print-out-paths)
mkdir -p "$work/providers/$name" "$work/data"
cp "$root/extras/$name/provider.yaml" "$work/providers/$name/provider.yaml"
printf 'version: ask.config/v1\n' > "$work/config.yaml"
export ASK_CONFIG="$work/config.yaml"
export ASK_PROVIDERS_DIRECTORY="$work/providers"
export XDG_DATA_HOME="$work/data"
export XDG_DATA_DIRS="$work/data"
export PATH="$core/bin:$provider/bin:$PATH"
unset ASK_PROVIDER ASK_PROVIDER_PATH ASK_PROVIDER_DEFAULT
unset GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_COMMON_DIR GIT_DIR GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_WORK_TREE
ask provider validate

git clone --quiet https://github.com/roshbhatia/changes.git "$work/changes"
git -C "$work/changes" checkout --quiet --detach 6147beb23c88864180be2cccdec9a52dd1a3a6fc
cd "$work/changes"
if [[ $mode == --record ]]; then
  vhs "$root/extras/$name/demo.tape" --output "$work/demo.gif"
  ffprobe -v error "$work/demo.gif"
  install -m 0644 "$work/demo.gif" "$root/extras/$name/demo.gif"
else
  model_args=()
  if [[ -n ${ASK_DEMO_MODEL:-} ]]; then
    model_args=(-m "$ASK_DEMO_MODEL")
  fi
  git show HEAD | ask "${model_args[@]}" --timeout 5m 'Explain this fix in four short sentences. Use relative file paths.'
fi
git diff --exit-code --quiet
