# fx

Replay an offline token parser review with fx.

The demo replays an offline response fixture. It does not contact a model or claim a new agent run.

## Install

```sh
brew install roshbhatia/tap/ask-provider-fx
nix profile add 'github:roshbhatia/ask#provider-fx'
```

Install the core utility separately, or select its all-provider bundle. Runtime tools still need their own credentials.

This adapter calls the Vercel coding agent from `vercel-labs/fx`. Install and authenticate that CLI before use. Homebrew’s `fx` formula is a different JSON viewer. The Nix package includes the coding agent. Run `ask provider validate` to check the installation.

## Demo

![Replay an offline token parser review with fx](demo.gif)

[Tape source](demo.tape) · [Task script](demo.sh)

Run `nix develop -c bash extras/fx/demo.sh` to run the task without recording.
Run `nix develop -c python3 hack/extra-demos.py fx` to record it.
