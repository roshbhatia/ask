# fm

Apple Foundation Models CLI.

## Install

```sh
brew install roshbhatia/tap/ask-provider-fm
nix profile add 'github:roshbhatia/ask#provider-fm'
```

Install the core utility separately, or select its all-provider bundle.

Requires macOS 27 or later on Apple Silicon with Apple Intelligence enabled. The adapter uses the system /usr/bin/fm; Nix does not install macOS or its model. Run `fm available` to check model readiness. Select it with `ask -p fm`, or set `provider.default=fm` in Ask settings. Model and light mode both use `system`. Schemas are sent as prompt instructions and Ask validates the result. Apple native schemas require extensions that arbitrary Ask schemas do not provide.

## Demo

Live recording pending. The previous canned-response recording was withdrawn.

Ask the on-device Apple model to summarize a Git change. [Tape source](demo.tape).

Run `nix develop -c bash extras/fm/demo.sh` with the real runtime installed and authenticated.
Add `--record` to capture the interactive session. This invokes the real service; output and timing vary.
