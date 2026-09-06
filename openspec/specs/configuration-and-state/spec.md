# configuration-and-state Specification

## Purpose

Define Ask's configuration precedence and local replay state.

## Requirements

### Requirement: Typed YAML configuration

Ask MUST load `ask.config/v1` YAML from `$XDG_CONFIG_HOME/ask/config.yaml` by
default. `ASK_CONFIG` MUST select a different file, and environment values with
the `ASK_` prefix MUST override YAML values.

#### Scenario: Environment overrides provider default

- **WHEN** YAML sets `provider.default` and `ASK_PROVIDER_DEFAULT` is set
- **THEN** Ask uses the environment value for that setting

### Requirement: Legacy configuration fallback

Ask MUST read the legacy `config.json` only when the default YAML path does not
exist and `ASK_CONFIG` is unset.

#### Scenario: Both YAML and legacy JSON exist

- **WHEN** both default configuration files exist
- **THEN** Ask reads YAML and ignores legacy JSON

### Requirement: Provider selection precedence

Ask MUST select the first available provider source in this order: explicit
`--provider`, provider-specific wrapper name, `ASK_PROVIDER`, then
`provider.default`. If none is set, Ask MUST open a picker only in an
interactive terminal.

#### Scenario: Script has no provider selection

- **WHEN** a non-interactive request has no provider from any source
- **THEN** Ask fails with instructions to select a provider

### Requirement: Configuration inspection

Ask MUST support setting, getting, and listing known configuration values. A
saved provider default MUST name a discovered provider unless the user unsets
the value.

#### Scenario: Environment shadows a saved value

- **WHEN** the user sets `provider.default` while `ASK_PROVIDER` is present
- **THEN** Ask saves the setting and warns that the environment still outranks it

### Requirement: Replay state

Ask MUST store the last input, prompt, and successful output below the XDG state
directory. It MUST expose those values for replay or inspection.

#### Scenario: Replay has no piped input

- **WHEN** the user runs `--replay` without standard input
- **THEN** Ask reuses the saved input and uses the supplied prompt or the saved prompt

### Requirement: Integration-supplied terminal snapshots

Ask MUST accept rolling terminal snapshots only through its hidden capture
interface and a file-name-safe `ASK_CAPTURE_ID`. Ask MUST remain independent of
the terminal or multiplexer that supplies them.

#### Scenario: Previous command output is requested

- **WHEN** an integration has captured two command boundaries for one identifier
- **THEN** `--last` sends the new lines between those snapshots without the trailing prompt
