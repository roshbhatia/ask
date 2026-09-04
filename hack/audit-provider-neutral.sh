#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_dir"

product_pattern='\b(claude|codex|goose|copilot|cursor|hermes|fx|antigravity|crush|wezterm|zmx)\b'
if rg --line-number --ignore-case --glob '!audit-provider-neutral.sh' \
  "$product_pattern" wrappers.txt internal cmd hack flake.nix .goreleaser.yaml .github/workflows; then
  printf 'Ask core contains provider product knowledge. Move each match under extras/.\n' >&2
  exit 1
fi

if find cmd -mindepth 2 -maxdepth 2 -type f -path 'cmd/ask-provider-*/*' -print -quit | grep -q .; then
  printf 'A central provider adapter still exists under cmd/.\n' >&2
  exit 1
fi
