# fx

fx coding agent CLI.

## Install

```sh
brew install roshbhatia/tap/ask-provider-fx
nix profile add 'github:roshbhatia/ask#provider-fx'
```

Install the core utility separately, or select its all-provider bundle.

This adapter calls the Vercel coding agent from `vercel-labs/fx`. Install and authenticate that CLI before use. Homebrew’s `fx` formula is a different JSON viewer. The Nix package includes the coding agent. Run `ask provider validate` to check the installation.

## Demo

Live recording pending. The previous canned-response recording was withdrawn.

Select fx in Ask and explain the Changes release archive fix. [Tape source](demo.tape).

Run `nix develop -c bash extras/fx/demo.sh` with the real runtime installed and authenticated.
Add `--record` to capture the interactive session. This invokes the real service; output and timing vary.
