# AGENTS.md

## Purpose

Inpakker is a Go CLI and terminal UI for organizing, validating, building and inspecting Microsoft Intune Win32 application packages.

It uses Microsoft's external `IntuneWinAppUtil.exe` for `.intunewin` packaging rather than implementing the package format itself.

Keep changes focused, predictable and compatible with the existing architecture. Prefer extending existing services and UI patterns over introducing parallel implementations.

Before making a non-trivial change, inspect the nearby implementation and tests first.

Do not refactor unrelated code as part of a focused task.

---

## Sources of truth

When documentation, assumptions or generated context disagree, use these sources:

* `go.mod` owns the required Go version and dependencies.
* `types/types.go` plus the config loaders and validators own persisted configuration contracts.
* Existing implementation and tests describe current behavior.
* `README.md` describes the released user-facing product at a high level.
* `docs/` contains detailed user-facing behavior and configuration.
* `CONTRIBUTING.md` owns contributor, branch and pull-request workflow.
* `SECURITY.md` owns vulnerability reporting guidance.
* `ROADMAP.md` describes future direction only.

Do **not** implement roadmap features, placeholder fields or speculative architecture unless the task explicitly asks for them.

When behavior changes intentionally, update implementation, tests and relevant documentation together.

---

## Repository map

* `main.go` — executable entry point.
* `cmd/` — Cobra commands, CLI behavior and TUI entry point. Keep commands thin.
* `cmd/output.go` — shared terminal-aware CLI output.
* `types/` — shared persisted and effective data contracts.
* `internal/config/` — configuration loading, validation, user-home resolution and typed edits.
* `internal/workspace/` — workspace/application discovery, scaffolding, target resolution and effective settings.
* `internal/buildcache/` — deterministic build fingerprints and local build state.
* `internal/packager/` — `IntuneWinAppUtil.exe` orchestration.
* `internal/unpacker/` — decoder execution and secure extraction.
* `internal/toolmanager/` — external-tool detection, configuration and downloads.
* `internal/process/` — injectable external-process execution.
* `internal/pathutil/` — safe relative-path handling.
* `internal/pathopener/` — injectable Explorer/folder opening.
* `internal/updater/` — release discovery and self-update behavior.
* `internal/tui/` — Bubble Tea application and TUI components.
* `docs/` — detailed end-user documentation.
* `.github/` and `.goreleaser.yaml` — CI/release automation.

Place new code in the existing package that owns the behavior whenever practical.

Do not create a new package/interface merely to avoid modifying an existing one.

---

## Core product contracts

### Workspaces and applications

Workspace and application UUIDs are stable identities and must not depend on display names or filesystem locations.

Portable workspace data includes workspace/application configuration and application source files.

Machine-specific data belongs under the Inpakker user home, not inside portable workspace configuration.

Removing a registered workspace must never delete the workspace files.

Relinking a workspace must preserve and verify workspace identity.

New workspaces may copy global defaults. Existing workspaces must not silently change when global defaults change.

Nested application groups are supported.

Use effective application settings consistently for:

* validation;
* build inputs;
* fingerprints;
* package lookup;
* inspection;
* TUI presentation.

Path-setting changes must never silently move existing files.

Preserve:

* safe-relative-path validation;
* Windows reserved-name handling;
* symlink/path-containment protections.

### Local versus portable state

User-local state may include:

* managed external tools;
* preferences;
* workspace registrations;
* build cache;
* update state;
* future credentials/tokens.

Portable workspace/application JSON must not contain:

* machine-specific executable paths;
* authentication tokens;
* credentials;
* secrets.

Future tenant or Intune integration may reference workspace identity, but do not add unused Graph/Entra/authentication fields before that functionality exists.

---

## CLI contracts

Commands should remain thin wrappers around reusable internal services.

Product operations should have scriptable CLI equivalents where practical.

JSON output must:

* never prompt;
* never animate;
* never emit ANSI;
* remain machine-readable.

Interactive behavior must not activate when stdin/stdout is unsuitable for interaction.

Where an operation supports interactive prompts, provide flags or non-interactive behavior suitable for automation.

Invocation/argument errors may show command usage.

Operational failures should remain concise and should not dump unrelated usage text.

Return non-zero status for actual operation failures.

Use `cmd/output.go` for shared CLI presentation instead of inventing command-specific styling.

---

## External tools and processes

External-process execution must remain injectable/testable.

Tests must not require a locally installed:

* `IntuneWinAppUtil.exe`;
* `IntuneWinAppUtilDecoder.exe`;
* future optional Windows tooling.

Do not treat unavailable Windows-only external tools on Linux CI as a product failure.

Explicitly configured tool paths must not be silently replaced by automatic discovery.

Tool downloads must use known upstream sources and remain explicit about required upstream license acceptance.

Do not execute arbitrary shell strings when direct process execution is available.

---

## Build and unpack behavior

Build cache is local machine state keyed by stable application identity.

A normal build may skip an application only when:

* its relevant inputs match the cached fingerprint; and
* the expected output artifact still exists.

`--force` rebuilds and may refresh cache state.

`--no-cache` must not read or write build-cache state.

Failed builds must not update successful cache records.

Unpacking must keep decoder execution isolated and extraction path-safe.

Terminology should remain consistent:

* **source** = packaging inputs;
* **output** = generated packages;
* **destination** = extracted/unpacked files.

Application install/uninstall commands are metadata and must not execute during normal packaging.

Execution of those commands belongs only to an explicit future/test workflow such as Windows Sandbox testing.

---

## TUI contracts

The TUI uses Bubble Tea/Bubbles/Huh/Lip Gloss.

`View()` must remain render-only. Do not perform:

* filesystem scans;
* hashing;
* process execution;
* networking;
* configuration writes

from rendering code.

Slow or external work must happen asynchronously through commands/services.

Results from asynchronous workspace operations must be tied to the originating workspace/request so stale results cannot overwrite newer state.

Long-running foreground operations must:

* prevent conflicting operations;
* support cancellation where practical;
* remain locked until the worker acknowledges cancellation;
* preserve completed/partial results where the existing workflow supports it.

Forms should allow normal editing/navigation before final validation. On failed submission:

* preserve entered values;
* show a useful error;
* focus or identify the invalid field.

Backspace must continue editing a focused input before being interpreted as navigation.

State must never rely on color alone.

Use display-width-aware alignment and preserve accessible/plain branding behavior.

### TUI UX direction

Keep the interface approachable with arrow keys and Enter.

Optional expert shortcuts may supplement, but never replace, normal navigation.

Prefer consistent conventions:

* `/` — filter/search the current list;
* `Enter` — primary/open action;
* `Esc` — cancel, clear or go back according to current context;
* `a` — contextual actions where applicable;
* `?` — contextual help;
* `:` — global command palette when implemented.

Do not hide an active filter.

Footer help, full help and actual key handling should use the same binding definitions rather than duplicating key meanings.

Prefer responsive list/detail or preview layouts where they improve usability, but preserve narrow-terminal operation.

Do not introduce Neovim-style modal editing, leader-key systems or required Vim knowledge.

---

## Configuration changes

Persisted configuration uses explicit schema versions.

Unsupported schema versions must fail clearly.

Configuration writes must remain atomic.

When changing a persisted contract:

1. update the canonical type;
2. update loading/validation;
3. update scaffolding/serialization;
4. update tests;
5. update relevant documentation.

Do not add migration or compatibility behavior unless the task explicitly requires it.

Do not add unused fields for future roadmap functionality.

---

## Error handling and implementation style

Wrap errors with useful operation/path context and preserve the underlying cause with `%w` when callers may inspect it.

Avoid new global mutable state when values can be scoped to a command, service or model.

Prefer the Go standard library unless a dependency clearly improves the implementation.

If dependencies intentionally change:

```sh
go mod tidy
```

and commit the corresponding `go.mod` and `go.sum` changes.

Do not hand-edit dependency checksums or generated artifacts.

Keep user-facing wording concise and consistent.

---

## Security and safety

Never commit:

* credentials or tokens;
* private signing keys;
* generated `.intunewin` files;
* decoded package contents;
* local test workspaces;
* build caches;
* locally built release binaries/archives.

Tests must use temporary locations and must not modify real:

* user configuration;
* workspace registrations;
* managed tools;
* PATH;
* Inpakker installations.

Treat external package/application metadata as untrusted input.

Validate filesystem paths before writing or extracting data.

Avoid writable exposure of real workspace directories to isolated test environments when a read-only mapping or temporary copy is sufficient.

---

## Development checks

After changing Go code, run:

```sh
gofmt -w <changed-go-files>
go test ./...
go vet ./...
go build ./...
```

Add focused tests for changed behavior.

Prefer table-driven tests for deterministic parsing, validation and decision logic.

Use injectable runners/services for external processes and network-dependent behavior.

If a required check cannot run because the local environment/toolchain is unavailable, state that clearly. Do not claim it passed.

Windows installer/uninstaller behavior has dedicated tests; do not test installer changes against a real user installation.

---

## Git and pull requests

Follow `CONTRIBUTING.md` for branch naming, release branches, Conventional Commits and pull-request workflow.

Do not hard-code the current active release branch into this file. Inspect the repository/CONTRIBUTING guidance when branch selection matters.

Keep commits focused.

Pull requests should explain:

* the user-visible or technical outcome;
* compatibility/configuration impact;
* tests/checks run.

Update tests and relevant user documentation in the same change as user-visible behavior.

Do not merge with failing required checks.

---

## Releases and updater

Release/update/signing changes are security-sensitive.

Before modifying release or updater behavior, inspect:

* `.github/workflows/release.yml`;
* `.goreleaser.yaml`;
* `internal/updater/`;
* installer/update signing code;
* relevant release tests.

Do not weaken checksum/signature verification or publish partial release artifacts.

Never commit private signing material.

Release implementation details belong in the release automation and development documentation rather than being duplicated extensively here.

---

## Documentation

Keep `README.md` as the friendly project landing page:

* what Inpakker is;
* key features;
* quick install;
* basic workflow;
* links to deeper documentation.

Avoid filling README with implementation/security internals unless a user must act on them.

Use:

* `docs/tui.md` for detailed TUI usage;
* `docs/configuration.md` for configuration/workspace behavior;
* `docs/troubleshooting.md` for troubleshooting;
* `ROADMAP.md` for future planned/exploratory functionality;
* `CONTRIBUTING.md` for development workflow.

Roadmap text is not released behavior.

When implementing a roadmap feature, update the roadmap and user documentation to reflect its actual state.

---

## Agent behavior

For non-trivial work:

1. inspect the relevant implementation and nearby tests;
2. identify the existing owning package/service;
3. preserve established contracts unless the task intentionally changes them;
4. implement the smallest coherent change;
5. add/update tests;
6. update relevant documentation;
7. run the applicable checks.

Do not:

* invent requirements not present in the task or repository;
* implement adjacent roadmap items “while here”;
* add speculative compatibility layers;
* perform unrelated cleanup;
* duplicate an existing service or source of truth;
* claim unsupported behavior has been tested.

If an existing pattern is clearly problematic, improve it only when that improvement is required for the requested change or is small and directly related.

---

## Maintaining this file

Keep `AGENTS.md` focused on durable repository-wide rules.

Do not add transient release numbers, dependency versions, implementation trivia or detailed feature documentation that can be read from the actual source of truth.

When a rule only applies to one subsystem and becomes substantial, prefer a scoped `AGENTS.md` in that subtree or dedicated development documentation instead of expanding this root file indefinitely.

