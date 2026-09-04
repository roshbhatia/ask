# ask

![Ask running a focused code review through a local agent](docs/ask.png)

![Ask animated code review](docs/ask.gif)

`ask` sends a prompt and optional standard input to any local agent harness that
implements its provider protocol.

The default Nix package contains no providers. Ask also supports typed JSON
output, reusable prompt and schema templates, and interactive provider
selection. `wrappers.txt` defines its provider-neutral short command names.

## Install
<!-- BEGIN GENERATED:install -->

Choose one Ask package. These alternatives must not be installed together:

~~~bash
# Core only. Install providers separately.
nix profile install github:roshbhatia/ask#ask

# Core plus the eight providers packaged by the root flake.
nix profile install github:roshbhatia/ask#full

# Core plus every maintained provider, including standalone extras.
nix profile install 'github:roshbhatia/ask?dir=extras#full'
~~~

Homebrew installs the provider-neutral core:

~~~bash
brew install roshbhatia/tap/ask
~~~

<!-- END GENERATED:install -->

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

## Terminal snapshots

`--last` is terminal-neutral. A shell integration sets a stable
`ASK_CAPTURE_ID` and pipes its current text into the hidden `--capture` flag at
command boundaries:

```bash
terminal-snapshot-command | ASK_CAPTURE_ID="$session_id" ask --capture
cargo build; ask --last 'Explain the compiler error and propose the smallest fix.'
```

Ask only rotates the supplied snapshots. The integration owns terminal
discovery and capture, so Ask does not depend on a terminal or multiplexer.

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
├── antigravity/{default.nix,main.go,provider.yaml}
├── claude/{default.nix,main.go,provider.yaml}
├── codex/{default.nix,main.go,provider.yaml}
├── copilot/{default.nix,main.go,provider.yaml}
├── crush/{default.nix,main.go,provider.yaml}
├── cursor/{default.nix,main.go,provider.yaml}
├── fx/{default.nix,main.go,runtime.nix,provider.yaml}
├── goose/{default.nix,main.go,provider.yaml}
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

Each provider owns its executable and manifest. Simple command providers reuse
the neutral Go adapter library under `extras/internal/textadapter`; streaming
providers implement the same protocol directly. Ask core does not install or
name either kind. Each executable reads one JSON request from standard input
and writes newline-delimited `provider/v1` events to standard output.

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

The generated provider table lists every package. Hermes owns its larger runtime
flake and remains separate from Ask core. Install it alone from that flake, or
use the all-provider `extras` flake:

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

## Command reference
<!-- BEGIN GENERATED:commands -->

### `ask`

Agents in your shell!

| Option | Description |
| --- | --- |
| `--get-config` `<value>` | print one setting and exit |
| `--json`, `-j` | answer in JSON, shape unspecified |
| `--last`, `-l` | send what the previous command printed, instead of stdin |
| `--list-config` | print every setting and exit |
| `--model`, `-m` `<value>` | which model to run; press tab for the ones this agent names |
| `--provider`, `-p` `<value>` | which installed provider to run |
| `--quiet`, `-q` | no progress output at all |
| `--replay` | rerun the last input, with this prompt or the last one |
| `--schema`, `-s` `<value>` | answer in JSON, in this shape: a field spec such as 'name:string, tags:[]string, count:int?', where a trailing question mark makes a field optional and a bar makes an enum, or @path to a JSON Schema file |
| `--schema-template` `<value>` | use a named schema template |
| `--set-config` `<value>` | write one setting, as KEY=VALUE, and exit |
| `--show-input` | print the last input and exit |
| `--show-last` | print what --last would send and exit |
| `--show-output` | print the last answer and exit |
| `--show-prompt` | print the last prompt and exit |
| `--template`, `-t` `<value>` | use a named prompt template |
| `--timeout` `<value>` | give up after this long |
| `--var` `<value>` | set one prompt template variable as NAME=VALUE; repeat as needed |

### `ask prompt`

Manage prompt templates

### `ask prompt list`

List prompt templates

### `ask prompt save`

Save the last prompt as a template

| Option | Description |
| --- | --- |
| `--description` `<value>` | describe when to use this prompt |
| `--schema` `<value>` | associate a default schema template |
| `--variable` `<value>` | declare NAME or NAME=DEFAULT; repeat as needed |

### `ask prompt show`

Print a prompt template

### `ask provider`

Inspect external inference providers

### `ask provider list`

List discovered providers

| Option | Description |
| --- | --- |
| `--json` | print JSON |

### `ask provider validate`

Validate provider manifests and dependencies

| Option | Description |
| --- | --- |
| `--json` | print JSON |

### `ask schema`

Manage schema templates

### `ask schema list`

List schema templates

### `ask schema save`

Save a field spec or JSON Schema file as a template

| Option | Description |
| --- | --- |
| `--description` `<value>` | describe the structured result |

### `ask schema show`

Print a schema template

<!-- END GENERATED:commands -->

## Provider and install reference
<!-- BEGIN GENERATED:providers -->

| Provider | Description | Actions | Install |
| --- | --- | --- | --- |
| `antigravity` | Antigravity CLI | `inference.generate`, `inference.models`, `provider.validate` | `nix profile install 'github:roshbhatia/ask#provider-antigravity'` |
| `claude` | Claude Code | `inference.generate`, `inference.models`, `provider.validate` | `nix profile install 'github:roshbhatia/ask#provider-claude'` |
| `codex` | Codex CLI | `inference.generate`, `provider.validate` | `nix profile install 'github:roshbhatia/ask#provider-codex'` |
| `copilot` | GitHub Copilot CLI | `inference.generate`, `provider.validate` | `nix profile install 'github:roshbhatia/ask#provider-copilot'` |
| `crush` | Crush CLI | `inference.generate`, `provider.validate` | `nix profile install 'github:roshbhatia/ask#provider-crush'` |
| `cursor` | Cursor Agent CLI | `inference.generate`, `inference.models`, `provider.validate` | `nix profile install 'github:roshbhatia/ask#provider-cursor'` |
| `fx` | fx coding agent CLI | `inference.generate`, `inference.models`, `provider.validate` | `nix profile install 'github:roshbhatia/ask#provider-fx'` |
| `goose` | Goose CLI | `inference.generate`, `provider.validate` | `nix profile install 'github:roshbhatia/ask#provider-goose'` |
| `hermes` | Hermes Agent CLI | `inference.generate`, `provider.validate` | `nix profile install 'github:roshbhatia/ask?dir=extras#provider-hermes'` |

<!-- END GENERATED:providers -->
