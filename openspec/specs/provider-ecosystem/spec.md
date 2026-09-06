# provider-ecosystem Specification

## Purpose

Define how Ask remains independent from inference products while discovering
optional provider packages.

## Requirements

### Requirement: Provider-neutral core

Ask core MUST identify integrations only through provider manifests and
protocol actions. The core package MUST NOT install a provider adapter or name
a provider product.

#### Scenario: Core is installed alone

- **WHEN** the core package runs with empty provider discovery paths
- **THEN** `ask provider list --json` returns an empty list
- **AND** an inference request reports that the user must select or install a provider

### Requirement: Ordered provider discovery

Ask MUST discover YAML or JSON manifests from the configured provider directory,
`ASK_PROVIDER_PATH`, XDG user data, executable-adjacent package data, and XDG
system data in that order. The first valid manifest for each provider name MUST
win.

#### Scenario: User manifest shadows packaged manifest

- **WHEN** a user manifest and a packaged manifest declare the same provider name
- **THEN** Ask uses the user manifest

#### Scenario: Earlier manifest is malformed

- **WHEN** an earlier root contains a malformed manifest and a later root contains a valid provider
- **THEN** discovery keeps the valid provider available
- **AND** provider validation reports the malformed manifest

### Requirement: Provider-owned extras

Each maintained integration MUST own its manifest, adapter, and Nix runtime
dependencies in its extras directory. Ask MUST provide separate provider
packages and aggregate packages without adding those dependencies to core.

#### Scenario: One provider package is installed

- **WHEN** the isolated package check exposes Ask core and one provider package
- **THEN** Ask discovers exactly that provider
- **AND** its adapter and declared runtime are present

#### Scenario: Complete extras are installed

- **WHEN** the extras flake `full` output is installed
- **THEN** Ask discovers every maintained provider package, including the standalone Hermes package

### Requirement: Provider manifest is the command source

A provider manifest MUST declare its version, name, description, base command,
and actions. It MAY declare host requirements, execution defaults, and
action-specific arguments or environment. Ask MUST render each argument and
environment value as an independent Go template.

#### Scenario: Provider needs product-specific arguments

- **WHEN** Ask invokes a declared action
- **THEN** the provider manifest supplies the complete product-specific argument mapping
- **AND** Ask core adds no product-specific behavior
