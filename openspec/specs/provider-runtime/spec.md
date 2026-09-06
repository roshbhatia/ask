# provider-runtime Specification

## Purpose

Define the validated process and wire contract between Ask and an external
inference provider.

## Requirements

### Requirement: Direct provider execution

Ask MUST execute a manifest's rendered argument vector directly without an
implicit shell. A relative command containing a path separator MUST resolve
from the manifest directory.

#### Scenario: Manifest ships a local adapter

- **WHEN** the manifest command is `./adapter`
- **THEN** Ask executes the adapter beside that manifest
- **AND** uses the request directory as the adapter working directory

### Requirement: Streaming generation protocol

For `inference.generate`, Ask MUST send one `provider/v1` JSON request on
standard input and read newline-delimited `provider/v1` events from standard
output. A successful stream MUST end with exactly one result event and a zero
process exit status.

#### Scenario: Provider exits after a result with failure

- **WHEN** an adapter emits a result and then exits nonzero
- **THEN** Ask reports a failed provider result

#### Scenario: Provider continues after its result

- **WHEN** an adapter emits another event after its result
- **THEN** Ask rejects the stream as a protocol failure

### Requirement: Protocol event validation

Ask MUST validate each event's protocol version, event kind, and result
placement. Ask MUST reject malformed JSON, unknown event types, oversized event
lines, missing results, and invalid result placement.

#### Scenario: Provider emits malformed JSON

- **WHEN** an adapter writes a non-JSON event
- **THEN** Ask ends the run with a failed result that identifies invalid JSON

### Requirement: Bounded invocation

Ask MUST bound inference work by the run timeout. Model discovery MUST use a
three-second bound, and provider contract validation MUST cap its action probe
at five seconds.

#### Scenario: Validation timeout is excessive

- **WHEN** a provider declares a validation timeout over five seconds
- **THEN** Ask limits the validation probe to five seconds

### Requirement: Deterministic provider validation

`ask provider validate [name]` MUST validate manifest structure, executable
requirements, required `inference.generate` and `provider.validate` actions,
every action template, and the provider's validation response. It MUST validate
one named provider or all discovered manifests and support JSON reports.

#### Scenario: Required action is absent

- **WHEN** a manifest omits `provider.validate`
- **THEN** validation reports the missing action and exits unsuccessfully

#### Scenario: Named provider is valid beside a broken sibling

- **WHEN** the user validates one valid provider and another manifest is malformed
- **THEN** the named validation checks only the requested provider

### Requirement: Structured answer enforcement

Ask MUST validate a structured result against the requested JSON Schema. It
MUST allow at most three rounds to answer one clarification or repair an invalid
shape.

#### Scenario: Non-interactive provider asks a question

- **WHEN** a provider returns only a clarification question without an interactive terminal
- **THEN** Ask prints the question and exits with status 3

#### Scenario: Final answer remains outside the schema

- **WHEN** the third provider result does not satisfy the requested schema
- **THEN** Ask fails instead of printing the result as a success
