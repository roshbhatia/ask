#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
spec=${PROVIDER_SPEC:?set PROVIDER_SPEC to a provider-spec checkout; nix develop exports it}
check=()
if [[ ${1:-} == "--check" ]]; then
  check=(--check)
fi

go run "$root/cmd/ask" generate --root "$root" "${check[@]}"
bash "$root/hack/audit-provider-neutral.sh"
for manifest in "$root"/extras/*/provider.yaml; do
  cue vet -d '#Manifest' "$spec/provider.cue" "$root/schema/narrow.cue" "$manifest"
done
for fixture in "$root"/schema/fixtures/*.yaml; do
  if cue vet -d '#Manifest' "$spec/provider.cue" "$root/schema/narrow.cue" "$fixture" 2>/dev/null; then
    echo "reject expected: $fixture" >&2
    exit 1
  fi
done
