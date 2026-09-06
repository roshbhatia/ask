# prompt-and-schema-templates Specification

## Purpose

Define reusable, typed prompt and structured-output templates.

## Requirements

### Requirement: XDG template storage

Ask MUST store prompt and schema templates as YAML below
`$XDG_CONFIG_HOME/ask/templates`, using separate `prompts` and `schemas`
directories. Saved files MUST include their JSON Schema directive.

#### Scenario: Prompt template is saved

- **WHEN** the user runs `ask prompt save review`
- **THEN** Ask atomically writes `prompts/review.yaml` with mode `0600`
- **AND** the document references the prompt-template schema

### Requirement: Prompt capture and association

`ask prompt save` MUST save the last executed prompt and MAY associate it with
an existing schema template. A prompt's associated schema MUST apply when the
run has no explicit schema choice.

#### Scenario: Explicit schema overrides the association

- **WHEN** a prompt template names a default schema and the user supplies `--schema`
- **THEN** Ask uses the explicit schema

### Requirement: Typed prompt variables

Version 2 prompt templates MUST render with Go `text/template` and MUST support
`string`, `bool`, `int`, `number`, and `json` variables. Ask MUST validate
variable declarations, defaults, supplied values, and template references
before invoking a provider.

#### Scenario: Required variable is missing in a script

- **WHEN** a non-interactive run omits a required template variable
- **THEN** Ask reports every missing variable without starting a provider

#### Scenario: Required variable is missing in a terminal

- **WHEN** an interactive run omits a required template variable
- **THEN** Ask prompts for the missing value and validates its declared type

### Requirement: Legacy prompt compatibility

Ask MUST continue to read `ask.prompt/v1` templates with literal
`{{variable}}` replacement semantics. New prompt saves MUST use
`ask.prompt/v2`.

#### Scenario: Legacy placeholder contains a dash

- **WHEN** a version 1 template declares and uses `{{repo-name}}`
- **THEN** Ask substitutes the supplied `repo-name` value literally

### Requirement: Reusable schema templates

Ask MUST save and load named `ask.schema/v1` YAML documents. The source schema
MUST accept either Ask's field specification or a JSON Schema file selected by
an `@` path.

#### Scenario: Field specification is saved

- **WHEN** the user saves `summary:string, risks:[]string, verdict:pass|revise`
- **THEN** Ask stores an object schema with required fields, an array item type, and an enum

### Requirement: Strict template documents

Ask MUST reject invalid names, unknown YAML fields, duplicate variables,
undeclared template variables, multiple YAML documents, mismatched declared
names, and references to missing default schemas.

#### Scenario: Template hides an undeclared value in a branch

- **WHEN** a Go template action references an undeclared variable inside a conditional branch
- **THEN** Ask rejects the template when it is saved
