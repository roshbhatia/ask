# ask

![Ask running a focused code review through a local agent](docs/ask.png)

![Ask animated code review](docs/ask.gif)

Recorded with Ask 0.7.1, its real Codex adapter, and `gpt-5.6-luna`. The recording pipes
[a release archive fix](https://github.com/roshbhatia/changes/commit/6147beb23c88864180be2cccdec9a52dd1a3a6fc)
into Ask, selects Codex interactively, and waits for its answer.
[Tape source](hack/ask.tape). Regeneration requires an authenticated Codex CLI;
answers and response times can differ between runs.

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

Add `-L` for bulk work a cheap model handles. It runs the light model the
provider's manifest declares and fails plainly on a provider that declares none.
`-m` names a model outright; the two do not combine.

```bash
git log -20 --format=%s | ask -p local-model -L \
  'Summarize these commits in one line each.'
```

## Templates

Save a reusable output schema. Then ask a question whose prompt contains the
variables you want to expose and save that last prompt:

```bash
ask schema save review-result \
  'summary:string, risks:[]string, tests:[]string, verdict:pass|revise'

ask -p local-model \
  'Review {{.repo}}. {{if .strict}}Reject untested migration risks.{{end}}'
ask prompt save code-review \
  --schema review-result \
  --variable repo:string \
  --variable strict:bool=true
```

Run the template with only the required value. `strict` uses its saved default.
The associated `review-result` schema is applied automatically:

```bash
git diff --staged | ask -p local-model \
  --template code-review \
  --var repo=payments-service
```

Repeat `--var NAME=VALUE` for each override. A declaration accepts
`NAME[:TYPE][=DEFAULT]`. Supported types are `string`, `bool`, `int`, `number`,
and `json`. Ask validates values before it starts a provider. Prompt templates
use Go `text/template`, so conditions such as `{{if .strict}}` use typed values.
The `json` template function renders a JSON variable without Go map syntax.

Ask prompts for missing required values when a terminal is available.
Non-interactive runs fail with the missing variable names.

Use `--schema-template NAME` to apply a schema without a prompt template. An
explicit `--schema` or `--schema-template` overrides a prompt template's default.

A template can also pin the provider and model it was written for:

```bash
ask prompt save triage --provider local-model --model light
```

This writes `provider:` and `model:` into the file. `model:` takes a literal id
or the role words `light` and `default`, which Ask resolves against the
provider's manifest when the template runs. `-p` on the command line beats the
template's provider, and `-m` or `-L` beat its model.

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
directory. Saved files include a YAML language-server directive. Their generated
schemas live at `schema/prompt-template.schema.json` and
`schema/schema-template.schema.json`. Ask still reads `ask.prompt/v1` files and
renders their `{{variable}}` placeholders with the original literal semantics.
New saves use `ask.prompt/v2` and Go template actions.

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
├── opencode/{default.nix,main.go,provider.yaml}
├── pi/{default.nix,main.go,provider.yaml}
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
  model: model-cli-large
  light: model-cli-small
```

`defaults.model` names the model a plain request runs and `defaults.light` the
cheap one `-L` selects. Both are ids the provider's own command accepts. Ask
resolves the role words `light` and `default` to them before it renders `argv`
or writes the request, so an adapter only ever sees a literal id. A provider
that declares no `light` makes `-L` an error rather than quietly running
something heavier.

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

## Typed evaluation

Evaluate stdin with an independently selected provider and model. Existing
text providers use schema-constrained generation. No specialist model is required.

```bash
ask --set-config evaluation.provider=local-model
ask --set-config evaluation.model=light

cat change.txt | ask evaluate \
  --boolean 'actionable=Does this describe a concrete next action?' \
  --choice 'kind=What kind of change is this?' \
  --choices 'kind=fix,feature,maintenance,other' \
  --score 'impact=How much behavior changes?' \
  --levels 'impact=none,limited,broad'
```

`-p` and `-m` override `evaluation.provider` and `evaluation.model`.
`ASK_EVALUATION_PROVIDER` and `ASK_EVALUATION_MODEL` override the file.
Generation settings, wrapper names, and `ASK_PROVIDER` do not select an evaluator.
The `light` and `default` model roles resolve through the selected provider's manifest.

Evaluation writes one `ask.evaluation/v1` JSON document to stdout. Diagnostics go
to stderr. Valid false answers exit 0; invalid input, invalid answers, provider
failures, and timeouts exit 1 without a partial result. Use the answer value for
shell predicates, and enable `pipefail` so an upstream failure remains a failure:

```bash
set -o pipefail
ask evaluate --boolean 'actionable=Is a next action identified?' < answer.txt |
  jq -e '.answers.actionable.value'
```

`method: structured` returns typed `value` fields without invented probabilities.
`method: native` preserves provider-supplied probabilities and distributions when
available. Boolean probability means P(true), not confidence in the chosen answer.
A native boolean value uses the 0.5 threshold; callers can apply their own threshold
to `probability`. Scores are numeric positions on the supplied levels, starting at
zero. The result includes the questions so downstream tools can interpret labels.

`--method auto` prefers the selected provider's declared `inference.evaluate`
action; otherwise it uses `inference.generate`. `--method native` and
`--method structured` require that capability. A native failure never silently
falls back to another method or provider. Inspect support without a model call:

```bash
ask provider capabilities
ask provider capabilities local-model
```

The default evaluation deadline is 30 seconds across provider work and schema
repairs. `--max-repairs 2` permits two repairs after the initial structured attempt;
`--max-repairs 0` disables them. Transport errors are not retried automatically.
Input is buffered to EOF before the provider deadline starts. Generation's
`--timeout` also spans all schema-repair attempts.

`--input auto` preserves a complete JSON value or treats stdin as text. Malformed
JSON-looking input fails; use `--input text` when braces are literal prose.
`--input json` requires JSON. Empty and binary input fail. Arrays remain one shared
state; Ask does not silently split JSONL or truncate input. Use `jq -s .` to
explicitly collect JSONL into one array.

### Generate, then evaluate

`--envelope` wraps generation output with its original prompt, input, provider,
and resolved model. It is separate from `--json`, which requests a JSON answer.
The normal answer and replay storage retain their existing format.

```bash
set -o pipefail
ask -p writer --envelope 'Summarize this change' < change.txt |
  ask evaluate -p reviewer -m light \
    --boolean 'faithful=Does the answer accurately summarize the input and follow the prompt?'
```

Pipe plain answers when only their content matters. Use an envelope when the
classification depends on the original request. Each stage also works alone.

### Reusable rubrics

A questions file uses stable IDs and explicit criteria:

```json
{
  "actionable": {"type": "boolean", "instructions": "Is a next action identified?"},
  "kind": {
    "type": "choice",
    "instructions": "What kind of change is this?",
    "choices": {"fix": "Corrects existing behavior", "other": "Anything else"}
  },
  "impact": {
    "type": "score",
    "instructions": "How much behavior changes?",
    "levels": ["none", "limited", "broad"]
  }
}
```

```bash
ask evaluate --questions review.json < change.txt
ask rubric save review review.json
ask rubric list
ask rubric show review
ask evaluate --rubric review < change.txt
```

Rubrics live beside prompt and schema templates at
`~/.config/ask/templates/rubrics/NAME.yaml`, with version `ask.rubric/v1`.
Files, rubrics, and inline questions combine; duplicate IDs and conflicting
criteria fail before a provider starts. Questions share the same unchanged state.
Dependent questions need separate calls.

### Evaluation provider contract

An evaluation-only manifest declares `inference.evaluate` and `provider.validate`.
Generation providers remain compatible. The native action receives one
`provider/v1` request containing `state`, `questions`, `model`, and `directory`.
It returns one JSON response with `version`, `answers`, and optional `metadata`.
Unlike generation, native evaluation does not stream events.

See `schema/protocol.evaluation-request.schema.json` and
`schema/protocol.evaluation-response.schema.json`. Each answer has a typed `value`.
Boolean answers may add `probability`; choice and score answers may add
`distribution`, keyed by their labels. Distributions must cover the supplied
labels and sum to one. Provider-specific usage and confidence belong in
`metadata`. An evaluation provider can be packaged independently of Ask core.

## Command reference
<!-- BEGIN GENERATED:commands -->

### `ask`

Agents in your shell!

| Option | Description |
| --- | --- |
| `--envelope` | emit a JSON envelope containing prompt, input, answer, provider and model |
| `--get-config` `<value>` | print one setting and exit |
| `--json`, `-j` | answer in JSON, shape unspecified |
| `--last`, `-l` | send what the previous command printed, instead of stdin |
| `--light`, `-L` | run the provider's light model, the cheap one it declares for bulk work |
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

### `ask evaluate`

Evaluate typed questions against stdin

| Option | Description |
| --- | --- |
| `--boolean` `<value>` | ID=question for a boolean judgment; repeatable |
| `--choice` `<value>` | ID=question for a categorical judgment; repeatable |
| `--choices` `<value>` | ID=label,label options for a choice question |
| `--input` `<value>` | stdin format: auto, text, or json |
| `--levels` `<value>` | ID=low,middle,high ordered levels for a score question |
| `--max-repairs` `<value>` | maximum structured-output repair attempts; 0 disables repair |
| `--method` `<value>` | auto, native, or structured; auto prefers advertised native evaluation |
| `--model`, `-m` `<value>` | evaluation model |
| `--provider`, `-p` `<value>` | evaluation provider; independent of generation settings |
| `--questions` `<value>` | JSON file containing named questions |
| `--rubric`, `-r` `<value>` | saved evaluation rubric |
| `--score` `<value>` | ID=question for an ordered score; repeatable |
| `--timeout` `<value>` | total provider deadline including schema repairs |

### `ask prompt`

Manage prompt templates

### `ask prompt list`

List prompt templates

### `ask prompt save`

Save the last prompt as a template

| Option | Description |
| --- | --- |
| `--description` `<value>` | describe when to use this prompt |
| `--model` `<value>` | pin the model: an id the provider accepts, or light or default; -m and -L on the run override it |
| `--provider` `<value>` | pin the provider to run; -p on the run overrides it |
| `--schema` `<value>` | associate a default schema template |
| `--variable` `<value>` | declare NAME[:TYPE][=DEFAULT], where TYPE is string, bool, int, number, or json; repeat as needed |

### `ask prompt show`

Print a prompt template

### `ask provider`

Inspect external inference providers

### `ask provider capabilities`

Print provider capabilities as JSON without model calls

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

### `ask rubric`

Manage reusable evaluation questions

### `ask rubric list`

List saved rubrics

### `ask rubric save`

Save questions from a JSON file

### `ask rubric show`

Print rubric questions as JSON

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
| `devin` | Devin CLI | `inference.generate`, `provider.validate` | `nix profile install 'github:roshbhatia/ask#provider-devin'` |
| `fx` | fx coding agent CLI | `inference.generate`, `inference.models`, `provider.validate` | `nix profile install 'github:roshbhatia/ask#provider-fx'` |
| `goose` | Goose CLI | `inference.generate`, `provider.validate` | `nix profile install 'github:roshbhatia/ask#provider-goose'` |
| `hermes` | Hermes Agent CLI | `inference.generate`, `provider.validate` | `nix profile install 'github:roshbhatia/ask?dir=extras#provider-hermes'` |
| `opencode` | opencode CLI | `inference.generate`, `inference.models`, `provider.validate` | `nix profile install 'github:roshbhatia/ask#provider-opencode'` |
| `pi` | pi coding agent CLI | `inference.generate`, `provider.validate` | `nix profile install 'github:roshbhatia/ask#provider-pi'` |

<!-- END GENERATED:providers -->
