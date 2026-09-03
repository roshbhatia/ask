# ask

![Ask running a focused code review through a local agent](docs/ask.png)

![Ask animated code review](docs/ask.gif)

`ask` sends a prompt and optional standard input to any local agent harness that
implements its provider protocol.

The default Nix package contains no providers. Ask also supports typed JSON
output, reusable prompt and schema templates, and interactive provider
selection. `wrappers.txt` defines its provider-neutral short command names.

Pipe data when the question needs context:

```bash
git diff --staged | ask -p local-model \
  --schema 'summary:string, risks:[]string, safe_to_merge:bool' \
  'Review this patch for correctness and migration risk.'
```

## Templates

Save a reusable output schema. Then ask a question whose prompt contains the
variables you want to expose and save that last prompt:

```bash
ask schema save review-result \
  'summary:string, risks:[]string, tests:[]string, verdict:pass|revise'

ask -p local-model 'Review {{repo}} with emphasis on {{focus}}.'
ask prompt save code-review \
  --schema review-result \
  --variable repo \
  --variable focus=correctness
```

Run the template with only the required value. `focus` uses its saved default.
The associated `review-result` schema is applied automatically:

```bash
git diff --staged | ask -p local-model \
  --template code-review \
  --var repo=payments-service
```

Repeat `--var NAME=VALUE` for each override. Ask prompts for missing required
values when a terminal is available. Non-interactive runs fail with the missing
variable names.

Use `--schema-template NAME` to apply a schema without a prompt template. An
explicit `--schema` or `--schema-template` overrides a prompt template's default.

```bash
ask prompt list
ask prompt show code-review
ask schema list
ask schema show review-result
```

Templates are ordinary YAML files:

```text
~/.config/ask/templates/
├── prompts/code-review.yaml
└── schemas/review-result.yaml
```

This keeps templates reviewable and portable. Set `XDG_CONFIG_HOME` to move the
directory.

Generate shell completions with `ask completion bash`, `zsh`, `fish`, or `nu`.

## Providers

Ask discovers integrations from `~/.config/ask/providers/<name>/provider.yaml`,
then each provider root in `ASK_PROVIDER_PATH`, `XDG_DATA_HOME`, and
`XDG_DATA_DIRS`. Flat manifest files also work for compatibility. The first
manifest with a given name wins. A release archive also discovers an adjacent
`providers` directory when one is present.

Each integration owns one directory. The manifest maps commands. The Nix file
packages its adapter and runtime dependencies:

```text
extras/
├── antigravity/{default.nix,provider.yaml}
├── claude/{default.nix,main.go,provider.yaml}
├── codex/{default.nix,main.go,provider.yaml}
├── copilot/{default.nix,provider.yaml}
├── crush/{default.nix,provider.yaml}
├── cursor/{default.nix,provider.yaml}
├── fx/{default.nix,runtime.nix,provider.yaml}
├── goose/{default.nix,provider.yaml}
└── hermes/{flake.nix,main.go,provider.yaml}
```

A provider manifest declares a command and the actions it supports. Each
argument and environment value is an independent Go template. Ask executes the
rendered argument vector directly. It never inserts a shell.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/roshbhatia/ask/main/schema/provider.schema.json
version: provider/v1
name: local-model
description: A local one-shot model
command: [ask-provider-local-model]
actions:
  inference.generate:
    description: Generate an answer
    argv:
      - --model-flag=--model
      - --prompt-flag=--prompt
      - --
      - model-cli
  provider.validate:
    description: Validate this adapter without model work
    argv:
      - --validate
      - --model-flag=--model
      - --prompt-flag=--prompt
      - --
      - model-cli
requires:
  commands: [model-cli]
defaults:
  timeout: 2m
```

The optional `ask-provider-text` adapter covers one-shot text commands. A
provider package can wrap it under its own command name, or supply a structured
adapter. Each adapter reads one JSON request from standard input and writes
newline-delimited `provider/v1` events to standard output. Harness-specific
arguments and output parsing stay outside Ask's core.

The generated wire schemas document each message:

```text
schema/protocol.request.schema.json
schema/protocol.event.schema.json
schema/protocol.models.schema.json
schema/protocol.validation.schema.json
```

`input` is plain JSON text. `inference.generate` streams events and ends with
one `result`. `inference.models` returns one models document.
`provider.validate` performs a deterministic local adapter probe and returns
`{"version":"provider/v1","status":"ok"}` without model work.

The flake publishes one package per built-in extra:

```bash
nix profile install github:roshbhatia/ask#ask github:roshbhatia/ask#provider-cursor
nix profile install github:roshbhatia/ask#full    # Ask plus root-flake providers
nix profile install github:roshbhatia/ask#extras  # Root-flake providers only
```

Hermes owns its larger runtime flake and remains separate from Ask core:

```bash
nix profile install 'github:roshbhatia/ask?dir=extras/hermes'
```

Each provider package includes its manifest, adapter, and CLI runtime. The
default `ask` package does not set `ASK_PROVIDER_PATH` and discovers nothing on
a clean XDG environment.

Inspect and validate the active integrations before a scripted run:

```bash
ask provider list
ask provider validate
ask provider validate local-model --json
```

Ask reads typed YAML from `~/.config/ask/config.yaml`. A legacy `config.json`
still works when the YAML file does not exist.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/roshbhatia/ask/main/schema/config.schema.json
version: ask.config/v1
provider:
  default: local-model
```

`ASK_CONFIG` selects another file. `ASK_PROVIDER_DEFAULT` overrides the YAML
setting, while the existing `ASK_PROVIDER` run-time choice still has higher
precedence.

## Development

```bash
nix develop
go test -race ./...
./hack/generate.sh --check
nix flake check
./hack/screenshots.sh
```
