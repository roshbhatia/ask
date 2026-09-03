#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
check=()
if [[ ${1:-} == "--check" ]]; then
  check=(--check)
fi

go run "$root" generate --root "$root" "${check[@]}"
"$root/hack/audit-provider-neutral.sh"
for manifest in "$root"/extras/*/provider.yaml; do
  cue vet -c -d '#Provider' "$root/schema/provider.cue" "$manifest"
done
