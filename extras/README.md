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
`#full` includes the eight root-packaged providers but excludes Hermes.

The flake discovers conforming directories instead of listing product names.
Every manifest must pass `schema/provider.cue`. Every package must also pass
its isolated closure check. That check exposes only Ask core and one provider
package, discovers the manifest through `XDG_DATA_DIRS`, and validates both the
adapter and its CLI dependency.
