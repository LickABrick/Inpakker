# Contributing to Inpakker

Thanks for helping improve Inpakker.

Bug reports, documentation fixes, focused code contributions and well-scoped feature proposals are welcome.

Please keep changes small enough to review comfortably and avoid mixing unrelated cleanup with functional work.

---

## Before opening an issue

Before reporting a problem:

1. Check whether the issue already exists.
2. Confirm the problem occurs on the latest released version when practical.
3. Run:

```powershell
inpakker doctor
```

or:

```powershell
inpakker doctor --json
```

4. Remove sensitive information before sharing output.

Do not include:

* customer names;
* tenant identifiers;
* credentials or tokens;
* proprietary application packages;
* private application source;
* sensitive logs;
* personal data.

Security vulnerabilities should not be reported through a public issue.

See [SECURITY.md](SECURITY.md).

---

## Development setup

Inpakker is written in Go.

Use the Go version declared in:

```text
go.mod
```

Clone the repository and restore dependencies normally:

```sh
git clone https://github.com/LickABrick/Inpakker.git
cd Inpakker
go mod download
```

Build:

```sh
go build ./...
```

Run the CLI/TUI from source:

```sh
go run .
```

---

## Required checks

After changing Go code, run:

```sh
gofmt -w <changed-go-files>
go test ./...
go vet ./...
go build ./...
```

If dependencies were intentionally changed, also run:

```sh
go mod tidy
```

and include both `go.mod` and `go.sum` changes.

Do not claim a check passed if it could not be run.

If your local toolchain or operating system prevents a specific check, mention that clearly in the pull request.

---

## Tests

Add or update tests when behavior changes.

Prefer focused unit tests for:

* parsing;
* validation;
* target resolution;
* path handling;
* deterministic decision logic;
* configuration behavior;
* TUI state transitions;
* process orchestration.

Table-driven tests are preferred where they keep deterministic logic readable.

External process execution should remain injectable or otherwise testable without depending on software installed on the development machine.

Tests must not require locally installed copies of:

* `IntuneWinAppUtil.exe`;
* `IntuneWinAppUtilDecoder.exe`;
* future optional Windows tooling.

Tests must not modify real user state.

Use temporary locations for:

* `INPAKKER_HOME`;
* workspace registrations;
* managed tools;
* installation directories;
* build state;
* update state.

Installer/uninstaller tests must never modify a developer's real PATH or Inpakker installation.

---

## Platform expectations

Windows AMD64 is the supported product target.

Much of the Go code and test suite is intentionally portable enough to run on Linux CI as well.

Do not treat the absence of Windows-only external tools on Linux as a product failure.

Windows-specific integration behavior should be isolated so that the rest of the test suite remains portable.

---

## Branches

`master` contains released, production-ready code.

Development is prepared on version branches such as:

```text
release/vX.Y
```

Create a short-lived branch from the applicable active development branch.

Use names such as:

```text
feature/<short-name>
fix/<short-name>
docs/<short-name>
chore/<short-name>
refactor/<short-name>
```

Examples:

```text
feature/sandbox-testing
fix/workspace-search
docs/readme-refresh
refactor/tui-bindings
```

Open the pull request back into the applicable development branch rather than directly into `master`, unless the repository workflow for that change explicitly says otherwise.

Do not hard-code the currently active release branch into documentation intended to remain evergreen.

---

## Commits

Use Conventional Commit subjects:

```text
<type>(optional-scope): <imperative summary>
```

Common types:

```text
feat
fix
docs
test
refactor
build
ci
chore
perf
revert
```

Examples:

```text
feat(tui): add contextual command palette
fix(workspace): preserve active filter state
docs: simplify configuration guide
test(packager): cover cancelled process execution
```

For breaking changes, use `!` and/or a `BREAKING CHANGE:` footer where appropriate.

Keep commits focused.

Avoid combining:

```text
feature implementation
+ dependency cleanup
+ unrelated renaming
+ formatting unrelated files
```

into one commit or pull request.

---

## Pull requests

A pull request should explain:

* what changed;
* why it changed;
* the user-visible or technical outcome;
* whether configuration or compatibility is affected;
* which tests/checks were run.

Update tests and relevant documentation in the same pull request as the behavior they describe.

A pull request should not be merged while required checks are failing.

Draft pull requests are encouraged for incomplete work that needs review or discussion.

---

## Architecture

Before introducing a new package, service or abstraction:

1. inspect the existing implementation;
2. identify which package currently owns the behavior;
3. check nearby tests;
4. prefer extending the existing design when it remains clear.

Do not create duplicate implementations for functionality already owned by another package.

Keep Cobra commands in `cmd/` thin and move reusable behavior into internal services.

Shared persisted data contracts belong in `types/`.

Configuration loading and persistence belong in `internal/config/`.

For broader architectural context, see:

```text
AGENTS.md
docs/development/architecture.md
```

when those files are present.

---

## Configuration compatibility

Persisted files use explicit schema versions.

When changing a persisted configuration contract:

1. update the canonical type;
2. update loading and validation;
3. update scaffolding and serialization;
4. update tests;
5. update documentation.

Do not add compatibility layers or migrations unless the change explicitly requires them.

Do not add unused fields for planned roadmap features.

Portable workspace configuration must not contain:

* credentials;
* authentication tokens;
* machine-specific secret material.

---

## External tools

Inpakker integrates with external tools such as Microsoft's Win32 Content Prep Tool and the optional package decoder.

Do not commit those executables to the repository unless the project explicitly changes its distribution model.

Tests should use mocks, fixtures or local test servers instead of downloading real upstream tools.

Tool installation/download behavior must remain explicit about upstream licenses.

---

## Security

Never commit:

* credentials;
* tokens;
* private signing keys;
* proprietary application content;
* generated `.intunewin` packages;
* decoded package contents;
* local test workspaces;
* local build caches;
* release binaries or archives.

Treat external application/package metadata as untrusted input.

Preserve path validation, containment checks and safe archive extraction behavior.

Security-sensitive release and updater changes require extra review.

See [SECURITY.md](SECURITY.md).

---

## Documentation

Use the documentation files for distinct purposes:

* `README.md` — friendly project overview, installation and quick start;
* `ROADMAP.md` — planned and exploratory future functionality;
* `docs/tui.md` — current terminal-interface usage;
* `docs/configuration.md` — current configuration behavior;
* `docs/troubleshooting.md` — problem-oriented help;
* `SECURITY.md` — vulnerability reporting and release integrity;
* `AGENTS.md` — durable repository-wide instructions for coding agents.

Current documentation should describe released or implemented behavior.

Future functionality belongs in `ROADMAP.md`.

Do not document roadmap features as if they already exist.

---

## Releases

Maintainers prepare releases from the applicable version branch and merge completed release work into `master`.

Release tags use Semantic Versioning:

```text
vMAJOR.MINOR.PATCH
```

Release automation is defined by the repository's GitHub Actions workflows and GoReleaser configuration.

Do not manually commit generated release artifacts.

Release/update/signing changes are security-sensitive; inspect the existing automation and updater tests before changing them.

Historical release-specific behavior belongs in:

```text
docs/releases/
```

Do not rewrite old release notes to match newer behavior.

---

## Questions and proposals

For a significant feature, open an issue or discussion before investing in a large implementation when possible.

Good proposals explain:

* the user problem;
* the intended workflow;
* what is intentionally out of scope;
* compatibility impact;
* whether the feature changes portable configuration.

Keep roadmap direction in mind, but do not implement adjacent roadmap features unless they are part of the agreed scope.
