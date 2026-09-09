#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
if [[ ${1:-} == --record && $# == 1 ]]; then
  exec bash "$root/extras/codex/record-main.sh"
fi
export ASK_DEMO_MODEL=gpt-5.6-luna
exec bash "$root/hack/demo-extra.sh" codex "$@"
