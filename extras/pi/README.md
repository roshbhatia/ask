# pi

pi coding agent CLI.

## Install

```sh
brew install roshbhatia/tap/ask-provider-pi
nix profile add 'github:roshbhatia/ask#provider-pi'
```

Install the core utility separately, or select its all-provider bundle.

This adapter calls `pi`. Install and authenticate that CLI before use. The Nix package includes its runtime. Run `ask provider validate` to check the installation.

## Demo

Live recording pending. The previous canned-response recording was withdrawn.

Select pi in Ask and explain the Changes release archive fix. [Tape source](demo.tape).

Run `nix develop -c bash extras/pi/demo.sh` with the real runtime installed and authenticated.
Add `--record` to capture the interactive session. This invokes the real service; output and timing vary.
