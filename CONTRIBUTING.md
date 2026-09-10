# Contributing to Inpakker

Thank you for helping improve Inpakker. Bug reports, documentation fixes, and
focused code contributions are welcome.

## Before opening an issue

- Search existing issues and confirm the problem occurs on the latest release.
- Run `inpakker doctor` and remove sensitive information from its output.
- Do not upload proprietary packages, application source, credentials, tenant
  identifiers, or private logs.
- Report security vulnerabilities privately according to [SECURITY.md](SECURITY.md).

## Development

Inpakker requires the Go version declared in `go.mod`.

After changing Go code, run:

```sh
gofmt -w <changed-go-files>
go test ./...
go vet ./...
go build ./...
```

Tests must not require installed copies of `IntuneWinAppUtil.exe` or
`IntuneWinAppUtilDecoder.exe`. Keep process execution injectable and cover path
behavior on both Linux and Windows where relevant.

## Branches and pull requests

Development is prepared on `release/vX.Y` version branches. Create a short-lived
`feature/`, `fix/`, `docs/`, or `chore/` branch from the applicable version
branch and open the pull request back into that branch. `master` contains only
released code.

Use Conventional Commit subjects:

```text
<type>(optional-scope): <imperative summary>
```

Examples include `feat: add application import`, `fix: preserve package output`,
and `docs: clarify decoder setup`.

Pull requests should explain the outcome, identify compatibility changes, list
the checks performed, and update user documentation alongside user-visible
behavior. Linux and Windows CI must pass before merging.

The active development line is `release/v0.4`. Its configuration/workspace
redesign is intentional; do not retain v0.3 compatibility code. Tests must set
`INPAKKER_HOME` to temporary storage and must not modify real user registrations,
managed tools or installation directories. Windows CI also runs
`powershell -File tests/installer.ps1`; these tests never modify real user PATH.

## Releases

Maintainers merge a completed version branch into `master`, create an annotated
Semantic Version tag on that merge, and push the tag. GitHub Actions verifies,
builds, signs, attests, and publishes release artifacts. Do not commit generated
executables, archives, checksums, signatures, or local workspaces.

Before v1.0.0, intentional breaking CLI, configuration and workspace changes may
ship in a MINOR release. After v1.0.0 they require a MAJOR increment. Keep the
installer's embedded public certificate aligned with the updater's certificate.
