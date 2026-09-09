# copilot

Replay an offline token parser review with copilot.

The demo replays an offline response fixture. It does not contact a model or claim a new agent run.

## Install

```sh
brew install roshbhatia/tap/ask-provider-copilot
nix profile add 'github:roshbhatia/ask#provider-copilot'
```

Install the core utility separately, or select its all-provider bundle. Runtime tools still need their own credentials.

This adapter calls `copilot`. Install and authenticate that CLI before use. The Nix package includes its runtime. Run `ask provider validate` to check the installation.

## Demo

![Replay an offline token parser review with copilot](demo.gif)

[Tape source](demo.tape) · [Task script](demo.sh)

Run `nix develop -c bash extras/copilot/demo.sh` to run the task without recording.
Run `nix develop -c python3 hack/extra-demos.py copilot` to record it.
