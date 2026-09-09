<p align="center">
  <img src="docs/assets/inpakker-logo.png" width="180" alt="Inpakker parcel box logo">
</p>

# Inpakker

[![Release](https://img.shields.io/github/v/release/LickABrick/Inpakker?display_name=tag&sort=semver)](https://github.com/LickABrick/Inpakker/releases/latest)
[![CI](https://github.com/LickABrick/Inpakker/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/LickABrick/Inpakker/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Inpakker is a Windows command-line and terminal application for organizing,
validating, packaging, and inspecting Win32 applications for Microsoft Intune.
It gives you a guided interface for everyday work while keeping every operation
available as a scriptable command.

## Features

- Guided workspace setup and application creation.
- Interactive terminal UI with search, details, and common actions.
- CLI commands suitable for PowerShell scripts and automation.
- Application groups for organizing larger packaging collections.
- Validation before packaging, with actionable error messages.
- Incremental builds that automatically skip unchanged applications.
- Batch build, validation, and unpack operations with progress reporting.
- JSON output for inventory and diagnostic automation.
- Optional `.intunewin` unpacking through a separately installed decoder.
- Signed, checksum-verified application updates from GitHub Releases.

## Requirements

- A 64-bit Windows system.
- [Microsoft Win32 Content Prep Tool](https://github.com/microsoft/Microsoft-Win32-Content-Prep-Tool),
  downloaded separately.
- Windows Terminal or another modern terminal is recommended for the interactive
  interface.

Inpakker calls Microsoft's `IntuneWinAppUtil.exe` to create packages. It does
not reimplement or redistribute Microsoft's packaging tool.

## Install

1. Download the Windows AMD64 ZIP and checksum manifest from the
   [latest release](https://github.com/LickABrick/Inpakker/releases/latest).
2. Verify the downloaded ZIP against the SHA-256 checksum manifest:

   ```powershell
   Get-FileHash .\inpakker_v0.2.0_windows_amd64.zip -Algorithm SHA256
   ```

3. Extract `inpakker.exe` to a directory of your choice.
4. Either run it using its full path or add that directory to your user `PATH`.

The version in the example filename changes with each release. Release checksum
manifests are cryptographically signed, and Inpakker verifies both the signature
and checksum before installing an update.

## Quick start

Open Windows Terminal in the directory where you want to keep your application
workspace and run:

```powershell
inpakker setup
```

The guided setup asks where `IntuneWinAppUtil.exe` is installed, creates the
workspace structure, and adds a harmless PowerShell example package. Check the
result and build the example:

```powershell
inpakker doctor
inpakker build example
```

Run `inpakker` without arguments to open the terminal interface.

## Common tasks

| Task | Command |
| --- | --- |
| Open the terminal interface | `inpakker` or `inpakker tui` |
| Initialize a workspace | `inpakker setup` |
| Check workspace health | `inpakker doctor` |
| Create an application | `inpakker new myapp` |
| List applications | `inpakker list` |
| View application details | `inpakker show myapp` |
| Validate all applications | `inpakker validate` |
| Build an application | `inpakker build myapp` |
| Build everything | `inpakker build --all` |
| Force a clean rebuild | `inpakker build --all --force` |
| Unpack an application package | `inpakker unpack myapp` |
| Check for an Inpakker update | `inpakker update --check` |

Commands that need an application will offer an interactive selector when used
in a terminal. Use `--no-input` where available to disable prompts in scripts.
Run `inpakker <command> --help` for all options.

## Terminal interface

The TUI provides fuzzy search, application status and details, guided app
creation, workspace diagnostics, refresh, validation, building, unpacking, and
application updates. The footer always shows the available keyboard shortcuts.

Every TUI operation has a CLI equivalent. Raw configuration editing and
destructive workspace operations are intentionally left to your editor and
version-control workflow.

For screen-reader-friendly forms, set either `INPAKKER_ACCESSIBLE=1` or
`ACCESSIBLE=1` before starting Inpakker. Colors automatically adapt to terminal
support and are omitted from redirected output.

## Workspace

An Inpakker workspace keeps the global configuration and all application source
files together:

```text
workspace/
  inpakker.config.json
  apps/
    example/
      app.config.json
      source/
        install.ps1
```

Applications may also be placed in group directories such as
`apps/browsers/firefox`. Paths in configuration files are relative to their
workspace or application directory.

Example global configuration:

```json
{
  "intunewinapputil": "C:\\Tools\\IntuneWinAppUtil.exe",
  "decoderPath": "C:\\Tools\\IntuneWinAppUtilDecoder.exe",
  "defaultOutputDir": "output",
  "appsDir": "apps",
  "muteIntuneWinAppUtil": false
}
```

See [Configuration](docs/configuration.md) for every setting and unattended
setup examples.

## Building applications

Build one or more applications or a complete group:

```powershell
inpakker build firefox 7zip
inpakker build browsers
inpakker build --all
```

Inpakker tracks the inputs used for a successful package. Running the same build
again reports unchanged applications as up to date. Use `--force` when you
intentionally need to recreate packages, or `--no-cache` for a one-off build
that should not use or change the saved build state.

## Optional unpacking

Unpacking is provided through Oliver Kieselbach's external
[IntuneWinAppUtilDecoder project](https://github.com/okieselbach/Intune/tree/master/IntuneWinAppUtilDecoder).
Review and obtain that third-party tool separately from its checked-in
[`bin/Release` folder](https://github.com/okieselbach/Intune/tree/master/IntuneWinAppUtilDecoder/IntuneWinAppUtilDecoder/bin/Release),
then set `decoderPath` to the full path of
`IntuneWinAppUtilDecoder.exe`. Inpakker does not bundle or redistribute it.

```powershell
inpakker unpack browsers/firefox
inpakker unpack C:\Packages\firefox.intunewin --destination C:\Decoded\firefox
inpakker unpack --all
```

Existing destinations are preserved unless `--force` is supplied. Decoder work
is isolated in a temporary directory, and unsafe paths in its resulting archive
are rejected.

## Updates

The CLI and TUI check GitHub Releases at most once every 24 hours. Ordinary CLI
commands only show a short notice when a newer stable version is available;
nothing is downloaded or installed automatically. No usage telemetry is
collected.

```powershell
inpakker update --check
inpakker update
inpakker update --yes
inpakker update --json
```

`inpakker update` shows the available version and release page, asks for
confirmation, downloads the matching release, verifies its cryptographic
signature and SHA-256 checksum, then replaces the executable with rollback
protection. Restart Inpakker after a successful update.

Set `INPAKKER_NO_UPDATE_CHECK=1` to disable automatic daily checks. Explicit
`inpakker update` commands continue to work. Automatic checks are skipped for
JSON output, redirected automation, help, completion, and development builds.

## Troubleshooting

Start with:

```powershell
inpakker doctor
```

It checks the workspace, external tools, and application configurations. Use
`inpakker doctor --json` when collecting the result in automation. See the
[troubleshooting guide](docs/troubleshooting.md) for common packaging, path,
permission, terminal, and update issues.

## Security and external tools

Please report suspected vulnerabilities privately as described in
[SECURITY.md](SECURITY.md). Do not include proprietary application packages,
credentials, tenant information, or sensitive logs in a public issue.

Microsoft Windows and Microsoft Intune are trademarks of Microsoft Corporation.
Inpakker is an independent project and is not affiliated with or endorsed by
Microsoft. External tools remain subject to their own terms and licenses.

## Contributing and license

Bug reports and focused contributions are welcome. See
[CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.

Inpakker is available under the [MIT License](LICENSE).
