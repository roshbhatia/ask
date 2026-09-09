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

| Extra | Task | Demo |
|---|---|---|
| [antigravity](antigravity/README.md) | Replay an offline token parser review with antigravity | [Tape](antigravity/demo.tape) |
| [claude](claude/README.md) | Replay an offline token parser review with claude | [Tape](claude/demo.tape) |
| [codex](codex/README.md) | Replay an offline token parser review with codex | [Tape](codex/demo.tape) |
| [copilot](copilot/README.md) | Replay an offline token parser review with copilot | [Tape](copilot/demo.tape) |
| [crush](crush/README.md) | Replay an offline token parser review with crush | [Tape](crush/demo.tape) |
| [cursor](cursor/README.md) | Replay an offline token parser review with cursor | [Tape](cursor/demo.tape) |
| [devin](devin/README.md) | Replay an offline token parser review with devin | [Tape](devin/demo.tape) |
| [fx](fx/README.md) | Replay an offline token parser review with fx | [Tape](fx/demo.tape) |
| [goose](goose/README.md) | Replay an offline token parser review with goose | [Tape](goose/demo.tape) |
| [hermes](hermes/README.md) | Replay an offline token parser review with hermes | [Tape](hermes/demo.tape) |
| [opencode](opencode/README.md) | Replay an offline token parser review with opencode | [Tape](opencode/demo.tape) |
| [pi](pi/README.md) | Replay an offline token parser review with pi | [Tape](pi/demo.tape) |

<!-- END GENERATED CATALOG -->
