# documentation-media-and-release Specification

## Purpose

Define the reproducible user documentation and release artifacts shipped by
Ask.

## Requirements

### Requirement: Immediate visual examples

The README MUST place a screenshot and an animated demonstration directly
below its title. The demonstration MUST invoke the real `ask` command against a
deterministic provider fixture.

#### Scenario: Reader opens the project page

- **WHEN** the README renders
- **THEN** the static screenshot and animated Ask run appear before the product description

### Requirement: Reproducible media

Ask MUST generate its screenshot with Freeze and its animation with VHS from
the declared Nix environment. A committed fingerprint MUST cover the media
script, tape, runtime sources, fixture, and package inputs.

#### Scenario: Runtime source changes without refreshed media

- **WHEN** the media freshness check computes a different source fingerprint
- **THEN** the check fails and directs the maintainer to regenerate the media

### Requirement: Real command demonstration

The VHS tape MUST type and run `ask` itself. It MUST wait for deterministic
answer text rather than use a separate demo executable as the visible command.

#### Scenario: Animation is regenerated

- **WHEN** the tape runs
- **THEN** the terminal shows a realistic code-review request and its provider result

### Requirement: Platform release archives

A tagged release MUST build static Ask archives for Apple Silicon Darwin, ARM
Linux, and x86-64 Linux. Each archive MUST include the license, README, shell
completions, and published schemas.

#### Scenario: Release tag is published

- **WHEN** a `v*` tag matches the version declared in `flake.nix`
- **THEN** GoReleaser publishes the platform archives and `checksums.txt`

### Requirement: Version gate

The release workflow MUST reject a tag whose version does not match the package
version.

#### Scenario: Tag and package version differ

- **WHEN** the release tag is not `v` followed by the `flake.nix` version
- **THEN** the release job does not run

### Requirement: Installation documentation

Generated documentation MUST describe the core-only Nix package, aggregate Nix
packages, standalone provider extras, and the Homebrew core package.

#### Scenario: User wants no bundled providers

- **WHEN** the user follows the core installation command
- **THEN** the installed Ask package contains no provider executables or manifests
