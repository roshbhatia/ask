#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
recorders=("$root"/extras/*/record-main.sh)
[[ ${#recorders[@]} == 1 && -f ${recorders[0]} ]]
exec bash "${recorders[0]}" "$@"
