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

To copy an existing installer while creating the application, use
`--setup-from "C:\Downloads\Firefox Setup.exe"` instead of `--setup-file`.
This copies only the selected file; add any companion files to the application's
`source` directory. Otherwise, place the installer there manually, then run:

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

To download the packaging tool or decoder, open Settings (`s`), select an
**External tools** row and press Enter → **Download from official source**.
Accept the upstream license terms to begin. You can also choose an existing
executable from the same menu. Attempting to build or unpack with a missing or
unavailable tool opens this setup menu directly.

On Applications, `/` searches, `Enter` opens details, `n` creates an application,
and `a` opens the selected application’s action menu.
`b` builds, `B` opens build options, `v` validates and `u` unpacks. `Ctrl+B`,
`Ctrl+V` and `Ctrl+U` operate on all applications. `o` opens the selected
application's root folder in Explorer; on Workspaces it opens the highlighted
workspace without switching. `r` refreshes in the background.

Creation uses one form with inherited source/output settings and an optional
group path with suggestions for existing groups. Forms use arrows, Tab and
Shift+Tab; Esc cancels. F2 browses workspace/tool paths or selects an installer
to copy during application creation. A scrollable review shows the application,
source, setup and output paths before creation; Esc returns to editing. Manual
path entry remains available. Backspace edits a focused input before navigating back.
Ctrl+C cancels an active operation.
Build, validation and unpack results stay in scrollable dialogs, with colored
status labels. Build tool output opens inside the dialog and returns to the result.
Result dialogs offer an **Actions** menu (`a`), `o` to open the output folder
and `r` to retry failed apps.
Press `L` to reopen the last operation result in the current session. Cancellation
waits for the worker to stop and keeps completed results. Packaging diagnostics
remain available even when “Show tool output” is off.

Tool downloads show their current phase and download size when available.
Settings offers an On/Off control for tool output and validates directory edits
before saving. Manual update checks and installation remain available when
automatic checks are disabled.

From the CLI, `inpakker open "Mozilla Firefox" --output` opens the application's
effective output folder. Retry failures by passing their targets to `build`,
`validate` or `unpack` again.

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
