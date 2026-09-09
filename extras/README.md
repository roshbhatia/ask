# Provider extras

Each directory owns one optional Ask provider. Its `provider.yaml` owns the
command mapping. Its `default.nix` owns the adapter and runtime dependencies.
The default Ask package does not install any directory from this tree.

Every directory owns its provider executable. `claude`, `codex`, and `hermes`
parse provider-specific output. The other providers reuse the neutral Go
adapter library under `extras/internal/textadapter`; their manifests remain the
source of command arguments. Each Nix package includes its executable and
declared CLI runtime.

Hermes owns a standalone flake because its runtime has a separate input graph.
This keeps Ask core independent from that graph while preserving a complete,
installable provider package.

The `extras` flake composes that standalone package with every root-flake
provider. Its `#full` output is the complete Ask installation. The root
`#full` includes the root-packaged providers but excludes Hermes.

The flake discovers conforming directories instead of listing product names.
Every manifest must pass the provider-spec contract plus `schema/narrow.cue`. Every package must also pass
its isolated closure check. That check exposes only Ask core and one provider
package, discovers the manifest through `XDG_DATA_DIRS`, and validates both the
adapter and its CLI dependency.

<!-- BEGIN GENERATED CATALOG -->

| Extra | Capability | Recording |
|---|---|---|
| [antigravity](antigravity/README.md) | Antigravity CLI | Pending |
| [claude](claude/README.md) | Claude Code | Pending |
| [codex](codex/README.md) | Codex CLI | [Demo](codex/README.md#demo) |
| [copilot](copilot/README.md) | GitHub Copilot CLI | Pending |
| [crush](crush/README.md) | Crush CLI | Pending |
| [cursor](cursor/README.md) | Cursor Agent CLI | Pending |
| [devin](devin/README.md) | Devin CLI | Pending |
| [fx](fx/README.md) | fx coding agent CLI | Pending |
| [goose](goose/README.md) | Goose CLI | Pending |
| [hermes](hermes/README.md) | Hermes Agent CLI | Pending |
| [opencode](opencode/README.md) | opencode CLI | Pending |
| [pi](pi/README.md) | pi coding agent CLI | Pending |

<!-- END GENERATED CATALOG -->
