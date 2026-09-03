# Provider extras

Each directory owns one optional Ask provider. Its `provider.yaml` owns the
command mapping. Its `default.nix` owns the adapter and runtime dependencies.
The default Ask package does not install any directory from this tree.

`claude`, `codex`, and `hermes` own Go adapters for provider-specific behavior.
The other manifests map their CLI subcommands to the provider-neutral
`ask-provider-text` adapter. Their Nix packages include that adapter and the
declared CLI runtime.

Hermes owns a standalone flake because its runtime has a separate input graph.
This keeps Ask core independent from that graph while preserving a complete,
installable provider package.

The flake discovers conforming directories instead of listing product names.
Every manifest must pass `schema/provider.cue`. Every package must also pass
its isolated closure check. That check exposes only Ask core and one provider
package, discovers the manifest through `XDG_DATA_DIRS`, and validates both the
adapter and its CLI dependency.
