# ask

![Ask running a focused code review through a local agent](docs/ask.png)

![Ask animated code review](docs/ask.gif)

`ask` sends a prompt and optional standard input to a local agent harness.

It supports Claude and Codex, typed JSON output, reusable prompt and schema
templates, and interactive provider selection. `wrappers.txt` defines its short
command names.

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

## Development

```bash
nix develop
go test -race ./...
nix flake check
./hack/screenshots.sh
```
