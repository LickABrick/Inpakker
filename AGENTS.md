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
- `internal/config/`: Schema validation, typed settings, user home resolution and configuration I/O.
- `internal/atomicfile/`: flushed atomic writes with Windows replacement support.
- `internal/toolmanager/`: deterministic global tool detection and official upstream downloads.
- `internal/pathopener/`: injectable shell-free Windows Explorer launch.
- `internal/buildcache/`: versioned build cache and deterministic input
  fingerprinting.
- `internal/cliui/`: reusable interactive CLI progress rendering.
- `internal/workspace/`: workspace discovery, inspection, validation, package
  lookup, registry/resolver, effective application settings and application scaffolding.
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
  `inventory.go` owns table/search state, `forms.go` defines single-page Huh dialogs, `manager.go` handles workspaces/settings/tools, `picker.go` embeds path browsing,
  `operations.go` runs cancellable services, and `render.go` composes the shell
  and pages using the centralized styles in `theme.go`.
- `types/types.go`: JSON-backed user, workspace and application configuration types.
- `README.md`: concise end-user installation, features, and common workflows.
- `docs/`: detailed end-user configuration, TUI, and troubleshooting references.
- `CONTRIBUTING.md` and `SECURITY.md`: public contribution and vulnerability
  reporting guidance.

Tests cover configuration validation, target discovery, command summaries,
incremental builds and read-only build-state inspection, usage errors,
onboarding, TUI routing/input/responsive states, packaging failures, scaffold
safety, decoder isolation, ZIP extraction safety, update caching, release
discovery, and signed update verification. There is no checked-in example
workspace. GitHub Actions runs Go tests on Linux and Windows and isolated PowerShell installer tests on Windows. GoReleaser
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
- The active development line is `release/v0.4`; dependency updates target it.
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

Installer/uninstaller tests run with `powershell -File tests/installer.ps1` on
Windows using temporary directories. Never test against a real user installation
or PATH. `install.ps1` embeds the same public signing certificate as the updater;
update both together if release trust changes. No private signing material belongs
in this repository. The uninstaller must preserve workspaces even with `-Purge`.

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

- before v1.0.0, a MINOR release may intentionally contain breaking CLI, configuration or workspace changes;
- after v1.0.0, incompatible public behavior requires a MAJOR increment;
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

Inpakker is installed once per user under `%LOCALAPPDATA%\Programs\Inpakker`.
User settings, managed tools, build state and update cache live under
`%LOCALAPPDATA%\Inpakker`; `INPAKKER_HOME` overrides this for development/tests.
Portable workspaces contain `inpakker.workspace.json`, an applications directory,
and `inpakker.app.json` beneath each application root. v0.4 deliberately replaces
the v0.3 formats; no compatibility aliases or automatic migrations are retained.

- Every workspace-dependent entry point uses the central resolver: explicit
  `--workspace`, `INPAKKER_WORKSPACE`, current/parent discovery, active registration,
  then no workspace. Discovered workspaces are never silently registered.
- Workspace/application UUIDs are generated internally and stable across location
  or display-name changes. Registrations contain UUID, path and last-opened time;
  portable workspace JSON owns the authoritative name. Names may contain spaces.
- `workspace create/add/use/show/list/remove/relink` manage registration and
  activation. Remove never deletes workspace files. Relink requires the same UUID.
  New workspaces copy global directory defaults; existing ones do not inherit
  subsequent global default changes.
- Discovery supports nested groups, stops at application roots and skips `.git`.
  CLI targets match canonical relative paths, unique human names, then groups.
  Ambiguous human names list canonical targets; there is no CLI fuzzy matching.
- `config.EffectiveApp` resolves source/output inheritance. Validation, build,
  fingerprints, package lookup, inspection and the TUI consume effective values.
  Path settings never move files automatically. Preserve safe relative paths,
  Windows reserved-name validation and symlink containment checks.
- `new` generates a directory slug until manually overridden and never overwrites
  an application directory. Rollback removes newly created empty directories only.
- Cache state lives in `state/workspaces/<UUID>/build-cache.json` under Inpakker
  home, keyed by application UUID. Build skips only matching inputs with existing
  artifacts. Rebuild (`--force`) updates cache; `--no-cache` neither reads nor
  writes cache. Failed builds do not update records.
- Tools are global. Detection uses configured path, PATH, managed tools, cwd,
  executable directory, then workspace root without recursive user-directory scans.
  Explicit configured paths are not silently replaced. Downloads require upstream
  license acceptance, official sources, cancellation, response/executable validation
  and atomic installation. Test downloads with local HTTP servers.
- `unpack` isolates decoder execution and securely extracts decoded ZIP output.
  Source directory means packaging inputs, output directory means generated
  packages, destination directory means extracted files. Install/uninstall commands
  are metadata only and must never execute during packaging.
- With no arguments, interactive invocation starts the TUI; redirected invocation
  prints help. No-workspace startup offers workspace creation/addition. Workspace
  management must work without configured external tools.
- TUI `View()` renders only memory: no scans, hashing, processes, networking or
  writes. Inventory is asynchronous and every result has workspace UUID and request
  generation. Workspace changes discard old inventory and reject stale results.
- Refresh, tool detection and update checks stay interactive; foreground packaging,
  unpacking and installation block conflicting operations and support cancellation.
- Input priority is window events, active operation, modal/form/input, page, global.
  Backspace edits focused inputs before navigation. Read scalar form submissions
  from Huh result keys, not pointers into copied Bubble Tea model values.
- `w` opens Workspaces, `s` Settings, `i` About and contextual `o` opens folders.
  About owns version/update information. Branding is decorative 📦 INPAKKER with
  a plain accessible fallback. Use display-width-aware terminal alignment.
- CLI `open` and TUI folder actions share an injectable path opener. Invoke Explorer
  directly with one path argument and return without waiting for it to close.
- Product operations have scriptable CLI equivalents; informational navigation
  such as About does not require duplicate CLI commands. TUI excludes application
  delete/rename and raw JSON editing, but supports constrained typed settings.
- JSON output must not prompt, animate or emit ANSI. Application failures produce
  concise detail lines and a count summary with nonzero status. Invocation/flag
  failures show usage; operational errors do not.
- Update checks share the user-local daily cache. Automatic failures are silent
  and do not affect command success. `INPAKKER_NO_UPDATE_CHECK=1` disables automatic
  checks. Explicit checks still work. Installation requires fresh metadata, a
  trusted signature and SHA-256 verification. Development builds cannot update.

## Configuration contracts

Canonical types live in `types/types.go`: `UserConfig`, `ToolConfig`,
`Preferences`, `WorkspaceDefaults`, `WorkspaceRegistration`, `WorkspaceConfig`,
`AppConfig` and `EffectiveAppConfig`. User/workspace/app/cache files use
`schemaVersion: 1`. Unsupported schemas fail clearly. Configuration writes are
atomic. Tool paths/preferences never belong in portable workspace JSON.

Global typed edits use known preference/default keys; workspace edits use name
and directory keys. CLI `--workspace-settings` selects config scope without
colliding with the global `--workspace <name-or-path>` selector. Update types,
loaders, scaffolding, tests and docs together when contracts change.

Future Intune integration can attach to workspace identity. Do not add unused
placeholder fields, Graph/Entra login or token storage. Credentials/tokens belong
in protected user-local storage, never portable JSON.

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
  never emit unconditional ANSI sequences. Use `✓`, `!`, `X`, `-`, `•`, `○`, and `—` for status markers. The box emoji is decorative branding only.
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
