# generated-interfaces Specification

## Purpose

Keep command help, documentation, schemas, and shell completion aligned with
the implemented interfaces.

## Requirements

### Requirement: One command metadata model

Ask MUST derive runtime help, README command reference, and shell completion
metadata from the Cobra command tree and shared completion model.

#### Scenario: Command option is added

- **WHEN** an option is present in the command tree
- **THEN** generated command documentation and completion metadata include it

### Requirement: Published shell completions

Ask MUST generate completions for Bash, Zsh, Fish, and Nushell. Packaged builds
MUST install the generated completion for each shell.

#### Scenario: Dynamic provider value is requested

- **WHEN** a shell completes `--provider`
- **THEN** the completion command returns names from the active provider discovery roots

#### Scenario: Model value is requested

- **WHEN** a selected provider advertises `inference.models`
- **THEN** model completion queries that provider within its bounded model-discovery call

### Requirement: Context-aware dynamic values

Completion generation MUST expose dynamic providers, models, prompt templates,
and schema templates. It MUST retain the selected provider from the command
line when completing models.

#### Scenario: Provider uses an equals argument

- **WHEN** the command line contains `--provider=local-model --model`
- **THEN** Ask queries `local-model` for the model candidates

### Requirement: Structured schema completion

Schema completion MUST offer a complete example for an empty value, field
types after a field separator, and normal path completion after `@`.

#### Scenario: JSON Schema file is started

- **WHEN** the completion value begins with `@`
- **THEN** Ask returns matching filesystem entries while preserving the `@` prefix

### Requirement: Generated JSON Schemas

Ask MUST generate JSON Schemas for configuration, provider manifests, prompt
templates, schema templates, and every provider wire document. Packaged builds
MUST include the generated JSON Schemas.

#### Scenario: Generated schema is stale

- **WHEN** `ask generate --check` finds content different from a committed schema
- **THEN** it exits unsuccessfully without rewriting the file

### Requirement: Generated README sections

Ask MUST generate the README install section, command reference, and provider
table. The provider table MUST derive from extras manifests instead of a
hard-coded provider list.

#### Scenario: Provider extra adds an action

- **WHEN** its manifest changes and generation runs
- **THEN** the README provider table shows the manifest's current actions
