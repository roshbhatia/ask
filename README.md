# ask

![Ask running a focused code review through a local agent](docs/ask.png)

![Ask animated code review](docs/ask.gif)

`ask` sends a prompt and optional standard input to any local agent harness that
implements its provider protocol.

The Nix package includes Claude Code and Codex providers. Ask also supports typed
JSON output, reusable prompt and schema templates, and interactive provider
selection. `wrappers.txt` defines its short command names.

Pipe data when the question needs context:

```bash
git diff --staged | ask -p codex \
  --schema 'summary:string, risks:[]string, safe_to_merge:bool' \
  'Review this patch for correctness and migration risk.'
```

## Templates

Save a reusable output schema. Then ask a question whose prompt contains the
variables you want to expose and save that last prompt:

```bash
ask schema save review-result \
  'summary:string, risks:[]string, tests:[]string, verdict:pass|revise'

ask -p codex 'Review {{repo}} with emphasis on {{focus}}.'
ask prompt save code-review \
  --schema review-result \
  --variable repo \
  --variable focus=correctness
```

Run the template with only the required value. `focus` uses its saved default.
The associated `review-result` schema is applied automatically:

```bash
git diff --staged | ask -p codex \
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
then each provider root in `ASK_PROVIDER_PATH`. Flat manifest files also work
for compatibility. The first manifest with a given name wins. The Nix
package adds its packaged providers to that path, so a user manifest can replace
one without changing Ask. A release archive also discovers its adjacent
`providers` directory after extraction.

Each integration owns one directory and one `provider.yaml` file:

```text
extras/
├── claude/provider.yaml
└── codex/provider.yaml
```

A provider manifest declares a command and the actions it supports. Each
argument and environment value is an independent Go template. Ask executes the
rendered argument vector directly. It never inserts a shell.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/roshbhatia/ask/main/schema/provider.schema.json
version: provider/v1
name: local-model
description: A local one-shot model
command: [ask-provider-text]
actions:
  inference.generate:
    description: Generate an answer
    argv: [model-cli, --model, "{{.Model}}", --prompt, "{{.Prompt}}"]
requires:
  commands: [model-cli]
defaults:
  timeout: 2m
```

The generic `ask-provider-text` adapter covers one-shot text commands. A
structured adapter can instead read one JSON request from standard input and
write newline-delimited `provider/v1` events to standard output. This keeps
harness-specific arguments and output parsing outside Ask's core.

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
  default: codex
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
