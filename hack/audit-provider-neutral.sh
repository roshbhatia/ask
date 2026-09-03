#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_dir"

product_pattern='\b(claude|codex|goose|copilot|cursor|hermes|fx|antigravity|crush)\b'
if rg --line-number --ignore-case --glob '!audit-provider-neutral.sh' \
  "$product_pattern" main.go wrappers.txt internal cmd hack flake.nix .goreleaser.yaml .github/workflows; then
  printf 'Ask core contains provider product knowledge. Move each match under extras/.\n' >&2
  exit 1
fi

if [[ -e cmd/ask-provider ]]; then
  printf 'The central provider multiplexer still exists at cmd/ask-provider.\n' >&2
  exit 1
fi
