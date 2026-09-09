# codex

Codex CLI.

## Install

```sh
brew install roshbhatia/tap/ask-provider-codex
nix profile add 'github:roshbhatia/ask#provider-codex'
```

Install the core utility separately, or select its all-provider bundle.

This adapter calls `codex`. Install and authenticate that CLI before use. The Nix package includes its runtime. Run `ask provider validate` to check the installation.

## Demo

![Select codex in Ask and explain the Changes release archive fix](../../docs/ask.gif)

Select codex in Ask and explain the Changes release archive fix. [Tape source](demo.tape).

Run `nix develop -c bash extras/codex/demo.sh` with the real runtime installed and authenticated.
Add `--record` to capture the interactive session. This invokes the real service; output and timing vary.
