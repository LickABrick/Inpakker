<p align="center">
  <img src="docs/assets/inpakker-logo.png" width="180" alt="Inpakker parcel box logo">
</p>

# Inpakker

Inpakker organizes, validates, builds and unpacks Win32 application packages for
Microsoft Intune. Install it once, then manage independent packaging workspaces
from a terminal or PowerShell scripts. Windows AMD64 is supported.

## Install

Run in PowerShell (no administrator privileges required):

```powershell
irm https://raw.githubusercontent.com/LickABrick/Inpakker/master/install.ps1 | iex
inpakker --version
inpakker
```

The installer verifies the release's signed checksum manifest and archive SHA-256,
installs into `%LOCALAPPDATA%\Programs\Inpakker`, and updates your user and current
process PATH. Re-running it installs the latest stable release without duplicate
PATH entries. Download the script to use `-Version`, `-InstallDir`, or `-Force`.

## Quick start

The terminal interface offers **Create workspace** and **Add existing workspace**.
Tools can be configured later. The equivalent CLI workflow is:

```powershell
inpakker workspace create D:\Intune\ADS --name "ADS Groep"
inpakker tools detect
inpakker tools install content-prep --accept-license
inpakker new "Mozilla Firefox" --group Browsers --setup-file "Firefox Setup.exe" --no-input
inpakker open "Mozilla Firefox"
```

Place the installer in the application's `source` directory, then run:

```powershell
inpakker validate
inpakker build "Mozilla Firefox"
```

Inpakker invokes the [Microsoft Win32 Content Prep Tool](https://github.com/microsoft/Microsoft-Win32-Content-Prep-Tool).
It downloads tools directly from their official upstream repositories after
license acceptance; tools are not bundled. You may instead configure your own:

```powershell
inpakker tools set content-prep C:\Tools\IntuneWinAppUtil.exe
```

## Workspaces

A workspace is an independent packaging environment with a human-readable name.
Run Inpakker from anywhere. Workspace resolution uses, in order:

1. `--workspace <name-or-path>`
2. `INPAKKER_WORKSPACE`
3. A workspace in the current directory or a parent directory
4. The globally active workspace

Discovered workspaces are not automatically registered.

```powershell
inpakker workspace add D:\Existing\Packaging
inpakker workspace list
inpakker workspace use "ADS Groep"
inpakker --workspace "ADS Groep" build --all
inpakker workspace relink "ADS Groep" E:\Intune\ADS
inpakker workspace remove "ADS Groep" --yes
```

Remove only removes registration. Workspace folders, sources, packages and
configuration remain intact. Relink requires the same workspace identity.

## Common tasks

| Task | Command |
| --- | --- |
| Open the interface | `inpakker` or `inpakker tui` |
| Inspect workspace | `inpakker workspace show` |
| Inspect applications | `inpakker list --json` |
| Show effective app settings | `inpakker show "Mozilla Firefox" --json` |
| Diagnose workspace and tools | `inpakker doctor` |
| Create application | `inpakker new "Mozilla Firefox" --setup-file setup.exe --no-input` |
| Build a group | `inpakker build Browsers` |
| Rebuild everything | `inpakker build --all --force` |
| Build without cache | `inpakker build --all --no-cache` |
| Open workspace folder | `inpakker open` |
| Open application folder | `inpakker open "Mozilla Firefox"` |
| Inspect global settings | `inpakker config show` |
| Change workspace output | `inpakker config set --workspace-settings outputDirectory packages` |
| Check updates | `inpakker update --check` |

Human names can contain spaces. Application directory names are generated from
names, or specified with `--directory-name`. Nested groups and canonical targets
such as `Microsoft/Office/microsoft-365-apps` are supported. Ambiguous names
produce an error listing the canonical targets.

**Build** skips unchanged applications and updates the cache on success.
**Rebuild** (`--force`) always packages and updates the cache.
**Build without cache** (`--no-cache`) neither reads nor writes the cache.

## Terminal interface

The shell shows **📦 INPAKKER**, the workspace name and its path. Press `w` for
Workspaces, `s` for Settings, `i` for About, and `?` for help. About contains the
version, update checks and update installation.

On Applications, `/` searches, `Enter` opens details, `n` creates an application,
`b` builds, `B` opens build options, `v` validates and `u` unpacks. `Ctrl+B`,
`Ctrl+V` and `Ctrl+U` operate on all applications. `o` opens the selected
application's root folder in Explorer; on Workspaces it opens the highlighted
workspace without switching. `r` refreshes in the background.

Creation uses one form with inherited source/output settings and a group
selector. Forms use arrows, Tab and Shift+Tab; Esc cancels. F2 browses paths in
workspace/tool dialogs, with manual entry always available. Backspace edits a
focused input before navigating back. Ctrl+C cancels an active operation.

Use `INPAKKER_ACCESSIBLE=1` or `ACCESSIBLE=1` for plain branding and accessible
CLI forms. Critical state always has a text/symbol label. See [TUI guide](docs/tui.md).

## Configuration and files

Machine-specific data lives under `%LOCALAPPDATA%\Inpakker` (`INPAKKER_HOME`
overrides this for development/tests):

```text
Inpakker/
  config.json
  tools/
  state/workspaces/<workspace UUID>/build-cache.json
  update/state.json
```

Portable workspace data:

```text
ADS/
  inpakker.workspace.json
  apps/Browsers/mozilla-firefox/
    inpakker.app.json
    source/Firefox Setup.exe
    output/Firefox Setup.intunewin
```

Each workspace owns its applications, source and output directory defaults.
Global defaults seed new workspaces. Applications inherit source/output settings
unless they explicitly override them. Changing directory settings never moves
existing files. See [Configuration](docs/configuration.md).

## Optional unpacking

Install Oliver Kieselbach's [Package decoder](https://github.com/okieselbach/Intune/tree/master/IntuneWinAppUtilDecoder)
from upstream, or configure an existing executable:

```powershell
inpakker tools install decoder --accept-license
inpakker unpack "Mozilla Firefox"
inpakker unpack C:\Packages\firefox.intunewin --destination C:\Decoded\firefox
```

Decoder execution is isolated and its ZIP output is checked for unsafe paths.
An existing extraction destination requires `--force`. Source means packaging
inputs, output means generated packages, and destination means extracted files.
Install/uninstall commands in application JSON are metadata, never executed.

## Updates and uninstall

Automatic update checks are limited to once per 24 hours. Installation requires
confirmation (`inpakker update`) or `inpakker update --yes` for automation.
`--check` and `--json` never install. Updates verify the trusted manifest
signature and artifact checksum. Set `INPAKKER_NO_UPDATE_CHECK=1` to disable
automatic checks; explicit checks remain available. Development builds cannot
replace themselves.

```powershell
irm https://raw.githubusercontent.com/LickABrick/Inpakker/master/uninstall.ps1 | iex
```

Uninstall removes the executable and its user PATH entry while preserving global
settings and workspaces. Download the script and run with `-Purge` to remove
global settings/tools/state as well. Workspace folders are always preserved;
purge refuses directories containing or overlapping workspace data.

## Help and license

Run `inpakker <command> --help` or consult [Troubleshooting](docs/troubleshooting.md).
See [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md) and the [MIT License](LICENSE).
Inpakker is independent of Microsoft; external tools retain their own licenses.
No Graph/Entra authentication, Intune uploads or telemetry are implemented.
