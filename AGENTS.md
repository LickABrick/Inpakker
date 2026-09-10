# AGENTS.md

## Project overview

Inpakker is a small Go CLI and terminal UI for organizing and packaging Win32
applications for Microsoft Intune. It wraps Microsoft's external
`IntuneWinAppUtil.exe`; it does not implement the `.intunewin` packaging format
itself.

The Go module is `github.com/LickABrick/inpakker` and currently targets Go
1.25.8. Cobra provides the CLI command structure. Bubble Tea, Bubbles, Huh, and
Lip Gloss provide the interactive terminal UI, forms, progress, and
terminal-aware presentation.

## Repository map

- `main.go`: executable entry point; delegates to `cmd.Execute`.
- `cmd/`: CLI commands, process exit behavior, and the TUI entry point. Keep
  commands thin and call reusable internal services.
- `cmd/output.go`: shared, terminal-aware command presentation.
- `internal/config/`: JSON file loaders.
- `internal/buildcache/`: versioned build cache and deterministic input
  fingerprinting.
- `internal/cliui/`: reusable interactive CLI progress rendering.
- `internal/workspace/`: workspace discovery, inspection, validation, package
  lookup, and application scaffolding.
- `internal/packager/`: reusable `IntuneWinAppUtil.exe` orchestration.
- `internal/unpacker/`: isolated external decoder execution and secure archive
  extraction.
- `internal/updater/`: daily GitHub release discovery, cached update state,
  signed checksum verification, archive validation, and rollback-aware
  executable replacement.
- `internal/process/`: injectable external-process runner.
- `internal/pathutil/`: cross-platform safe-relative-path validation.
- `internal/tui/`: Bubble Tea workspace interface. `model.go` orchestrates the
  application, `navigation.go` and `keymap.go` define routes/input precedence,
  `inventory.go` owns table/search state, `forms.go` defines staged Huh dialogs,
  `operations.go` runs cancellable services, and `render.go` composes the shell
  and pages using the centralized styles in `theme.go`.
- `types/types.go`: JSON-backed global and application configuration types.
- `README.md`: concise end-user installation, features, and common workflows.
- `docs/`: detailed end-user configuration, TUI, and troubleshooting references.
- `CONTRIBUTING.md` and `SECURITY.md`: public contribution and vulnerability
  reporting guidance.

Tests cover configuration validation, target discovery, command summaries,
incremental builds and read-only build-state inspection, usage errors,
onboarding, TUI routing/input/responsive states, packaging failures, scaffold
safety, decoder isolation, ZIP extraction safety, update caching, release
discovery, and signed update verification. There is no checked-in example
workspace. GitHub Actions runs tests on Linux and Windows, and GoReleaser
publishes tagged releases.

## Git workflow

Use Conventional Commits for commit subjects:

```text
<type>(optional-scope): <imperative summary>
```

Common types are `feat`, `fix`, `docs`, `test`, `refactor`, `build`, `ci`,
`chore`, `perf`, and `revert`. Use `!` and/or a `BREAKING CHANGE:` footer when a
change breaks compatibility. Keep commits focused, and do not combine unrelated
cleanup with a functional change.

Development follows version branches rather than merging feature work directly
into `master`:

- `master` represents released, production-ready code.
- Create a `release/vX.Y` branch for the next planned minor or major release.
  Patch-only release branches may use `release/vX.Y.Z` when they must be prepared
  independently of the next release line.
- Create short-lived branches such as `feature/<short-name>`,
  `fix/<short-name>`, `docs/<short-name>`, or `chore/<short-name>` from the
  applicable version branch.
- Open pull requests from short-lived branches into that version branch. Do not
  merge incomplete work merely to share it; use draft pull requests where
  appropriate.
- When the version branch is complete and verified, open a release pull request
  from it into `master`. The release PR is the point for reviewing the complete
  release notes, version, compatibility impact, and generated artifacts.
- Delete short-lived branches after merging. Delete version branches after the
  release unless they are intentionally retained for maintenance.
- Urgent production fixes should branch from `master`, target an appropriate
  patch release branch, and be carried forward into any active later version
  branch when applicable.

Pull requests should explain the user-visible or technical outcome, identify
breaking changes and config migrations, and state which checks were run. Update
tests and user documentation in the same PR as the behavior they cover. A PR
should not be merged with failing required checks.

## Development workflow

Run these checks after changing Go code:

```sh
gofmt -w <changed-go-files>
go test ./...
go vet ./...
go build ./...
```

Use `go test ./...` even while there are no explicit test files: it compiles all
packages. Add focused unit tests for new parsing, validation, discovery, or path
logic. Keep tests independent of installed `IntuneWinAppUtil.exe` and
`IntuneWinAppUtilDecoder.exe` binaries; inject or isolate process execution.

The packaging and unpacking integrations can only be exercised where their
configured Windows executables are available. Do not treat inability to run
those external executables on Linux as a product failure. Do not commit
generated Windows executables, `.intunewin` packages, decoded contents,
build caches, coverage output, or local test workspaces. A locally built
extensionless Unix binary is not currently ignored, so take care not to stage
one.

If the local Go toolchain itself is unavailable or broken, report that
separately and do not claim the checks passed.

## Releases and artifacts

Use Semantic Versioning (`MAJOR.MINOR.PATCH`):

- increment `MAJOR` for incompatible CLI, configuration, or workspace changes;
- increment `MINOR` for backward-compatible functionality;
- increment `PATCH` for backward-compatible fixes.

Release versions are identified by annotated Git tags named `vX.Y.Z`. A tag
must point to the release commit on `master`; do not release arbitrary feature
or version-branch commits.

The release workflow is implemented by `.github/workflows/release.yml` and
`.goreleaser.yaml`. Pushing a valid release tag runs formatting, tests, vetting,
and compilation before GoReleaser builds the supported binaries, creates
checksums, and publishes them to the matching GitHub Release. Release jobs must
fail instead of publishing a partial set when a required build or check fails.
Tags not matching `vX.Y.Z` (with optional SemVer prerelease/build metadata), or
whose commits are not contained in `master`, are rejected.

Release artifacts have stable, machine-readable names that include the project,
version, operating system, and architecture, for example
`inpakker_v1.2.3_windows_amd64.zip`. The supported release target is currently
Windows AMD64. Publish a checksum manifest, its detached ECDSA signature, and a
GitHub provenance attestation alongside the archives. The signing certificate
in `internal/updater/release-signing-cert.pem` is public; its matching private
key must exist only in secure maintainer storage and the encrypted
`INPAKKER_RELEASE_SIGNING_KEY` GitHub Actions secret. Do not commit release
binaries, archives, signatures, or private keys to the repository.

Tags and GitHub Releases are the update source of truth; avoid mutable version
labels such as `latest` inside filenames. The updater accepts stable releases
only and must require the versioned Windows archive, checksum manifest, and
trusted manifest signature before installation. The CLI exposes the embedded
release version through `inpakker --version`; development builds report `dev`
and may not replace themselves.

## CLI behavior and workspace model

Commands expect to run from an Inpakker workspace root. The global config file
is named `inpakker.config.json`. A typical workspace contains an apps root and
one `app.config.json` per package:

```text
workspace/
  inpakker.config.json
  apps/
    example/
      app.config.json
      source/
```

Preserve these current semantics unless a change explicitly intends to revise
them:

- `build` loads `inpakker.config.json` from the current working directory.
- `build --all` recursively finds `app.config.json` below the configured
  `appsDir` and skips directories whose path ends in `.git`.
- Named build targets are relative to `appsDir`. A target can be an app or a
  group containing apps as immediate child directories.
- Target, source, setup, and output paths may not escape their documented roots.
- An app's `outputDir` overrides the global `defaultOutputDir`; the fallback is
  `output`.
- `intunewinapputil` is the documented global JSON key. The legacy
  `intuneWinAppUtilPath` key is also accepted as a fallback.
- `muteIntuneWinAppUtil` controls whether the wrapped tool inherits stdout and
  stderr.
- `decoderPath` points to a separately installed
  `IntuneWinAppUtilDecoder.exe`; no decoder binary is embedded or distributed.
- `new` scaffolds under the configured `appsDir`, optionally beneath a safe
  group path, and must not overwrite an existing app config.
- `validate` recursively scans the configured `appsDir` and checks config
  fields, safe relative paths, source directories, and setup files. Invalid
  applications produce a non-zero exit status.
- `build` and `validate` print one detail line per failed application followed
  by a count summary. Build successes may use one concise check line; unchanged
  and missing targets are aggregated. Any application failure produces a
  non-zero exit status.
- `build` fingerprints configuration, source names/content, and packaging-tool
  identity. It skips only when the versioned cache matches and every recorded
  artifact still exists. `--force` bypasses the match and updates the cache;
  `--no-cache` bypasses cache reads and writes. Failed builds do not update it.
- `list` and `show` expose workspace/app inspection and offer JSON where
  applicable. `unpack` accepts app/group targets or direct `.intunewin` paths,
  isolates decoder side effects in a temporary directory, and securely extracts
  its decoded ZIP result.
- With no arguments, an interactive terminal opens the TUI; non-interactive
  invocation prints help. `tui` opens it explicitly. Every TUI action must have
  a non-interactive CLI command/flag equivalent. The TUI intentionally excludes
  delete, rename, and raw config editing.
- TUI rendering operates only on inspected in-memory inventory. Refresh it on
  initial load, explicit refresh, and after create/build/validate/unpack; never
  perform filesystem or network work from `View()`.
- TUI input priority is window events, active operation, modal/form/input, page,
  then global actions. Backspace belongs to focused inputs before route
  navigation, and Ctrl+C cancels an active operation without quitting the TUI.
- `setup` initializes the config and directories and creates a valid PowerShell
  example unless `--no-example` is used. `doctor` performs read-only workspace
  checks. A missing config encountered during interactive TUI startup launches
  setup rather than returning an unassisted file error.
- `update` works outside a workspace. `--check` never installs, `--yes` permits
  non-interactive installation, and `--json` is check-only and never prompts.
  The CLI and TUI share a per-user update cache and make at most one automatic
  GitHub check per 24 hours. Ordinary check failures stay silent, automatic
  checks never affect command success, and `INPAKKER_NO_UPDATE_CHECK=1` disables
  them. Installation always re-fetches release metadata and verifies the signed
  checksum before replacing the executable.

When changing path or discovery behavior, cover absolute/relative paths,
missing files, groups, nested apps, and platform-specific separators. Use
`filepath` rather than manual path concatenation.

## Configuration contracts

The canonical definitions are in `types/types.go`:

- Global: `intunewinapputil`, legacy `intuneWinAppUtilPath`, `decoderPath`,
  `defaultOutputDir`, `appsDir`, and `muteIntuneWinAppUtil`.
- App: `name`, `displayName`, `source`, `setupFile`, `installCommand`,
  `uninstallCommand`, and optional `outputDir`.

Keep JSON tags stable unless compatibility is deliberately being changed. If a
field, default, or accepted key changes, update the types, loaders/scaffolding,
tests, and README together. `installCommand` and `uninstallCommand` are reserved
for future use and are not part of the packaging invocation today.

## Implementation conventions

- Keep command definitions in `cmd/` and configuration I/O in
  `internal/config/`; shared data contracts belong in `types/`.
- Return errors from Cobra `RunE` handlers for command-level failures. Continue
  processing other apps only for failures intentionally scoped to one target.
- Wrap errors with useful operation and path context while preserving the cause
  with `%w` when callers may inspect it.
- Avoid adding global mutable command state when a value can be scoped to a
  command or passed to a helper. Existing package globals are not a requirement
  for new code.
- Keep user output concise and consistent with the existing info, warning,
  failure, and summary messages. Use the shared console in `cmd/output.go`.
  Lip Gloss styling must degrade cleanly for redirected/non-interactive output;
  never emit unconditional ANSI sequences. Use `✓`, `!`, `X`, and `-` as the
  standard success, warning, failure, and skipped/current markers; do not add
  emoji status markers.
- Invocation and flag errors must include command usage. Operational errors
  must remain concise and must not dump usage. Interactive prompts require a
  terminal and must have flag-based, `--no-input` alternatives. JSON output
  must never prompt or animate. Use Huh for forms and the shared CLI progress
  model for long, measurable work.
- Use standard-library functionality unless a dependency provides a clear
  benefit. Run `go mod tidy` after intentionally changing dependencies and
  include both `go.mod` and `go.sum` changes.
- Do not hand-edit generated artifacts or dependency checksums.

## Documentation expectations

Update `README.md` whenever user-visible commands, flags, defaults, workspace
layout, supported config keys, or platform requirements change. Examples should
be runnable and should use Windows syntax where they demonstrate the external
Intune utility, while ordinary Go development commands may remain portable.

Keep this file descriptive of the repository's actual workflow. If new tests,
CI, release tooling, or architectural layers are added, revise the relevant
sections rather than leaving stale instructions.

Keep the README focused on released end-user behavior. Put full configuration
and troubleshooting material under `docs/`; put development and release details
in `CONTRIBUTING.md` or this file. Do not expose dependency versions or internal
implementation details in the README unless an end user must act on them.
When the active version branch changes, update Dependabot's `target-branch` in
the same pull request so dependency updates continue to follow this workflow.

## Maintaining this file

Update `AGENTS.md` in the same pull request whenever a change affects how
contributors or agents should work, including changes to:

- repository structure, package ownership, or architectural boundaries;
- required local commands, tests, linters, CI checks, or supported toolchains;
- branch names, merge targets, commit conventions, or pull-request policy;
- release versioning, tagging, automation, supported build targets, or artifact
  naming;
- configuration compatibility rules, generated files, or documentation duties.

Do not update this file for an isolated implementation detail that does not
change the repository workflow or an enduring convention. Instructions should
describe the current state or clearly label an intended workflow that has not
yet been implemented. Remove obsolete guidance as part of the change that makes
it obsolete.
