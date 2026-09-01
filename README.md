# ask

`ask` sends a prompt and optional standard input to a local agent harness.

It supports Claude and Codex, typed JSON output, reusable configuration, and
interactive provider selection. `wrappers.txt` defines its short command names.

## Development

```bash
nix develop
go test -race ./...
nix flake check
```
