# Configuration

All persisted configurations use `schemaVersion: 1`. Unsupported versions and
unknown fields fail clearly. Writes encode a complete temporary file, flush it,
then atomically replace the active file.

## Global user settings

`%LOCALAPPDATA%\Inpakker\config.json` stores user/machine settings. `INPAKKER_HOME`
overrides the data root. On non-Windows development hosts the fallback is the OS
user configuration directory plus `Inpakker`; Windows is the supported product.

```json
{
  "schemaVersion": 1,
  "tools": {
    "contentPrepTool": { "path": "C:\\Tools\\IntuneWinAppUtil.exe" },
    "decoder": { "path": "" }
  },
  "preferences": { "showToolOutput": false },
  "workspaceDefaults": {
    "applicationsDirectory": "apps",
    "sourceDirectory": "source",
    "outputDirectory": "output"
  },
  "workspaces": [],
  "activeWorkspaceId": ""
}
```

Managed downloads also record `sourceUrl`, upstream commit in `version`, and
`sha256`. These record provenance; they do not claim vendor signing. Discovery
checks the configured path, PATH, managed tool storage, current directory,
Inpakker executable directory and workspace root, in that order. Only exact
filenames and the managed version-directory level are inspected. Invalid
explicit paths are preserved until you choose a replacement.

Registrations contain `id`, absolute `path`, and `lastOpenedAt`. Workspace JSON
owns the display name. An unavailable path remains registered and uses its
basename for display. `workspace relink` checks the UUID. `workspace remove`
changes only the registry.

## Workspace settings

Create `inpakker.workspace.json` with `workspace create`. Example:

```json
{
  "schemaVersion": 1,
  "id": "f13cb18e-1122-4344-8566-778899aabbcc",
  "name": "ADS Groep",
  "applicationsDirectory": "apps",
  "sourceDirectory": "source",
  "outputDirectory": "output"
}
```

UUIDs are generated automatically and identify machine-local cache state.
They remain stable when the workspace name or location changes. Tools do not
belong in this portable file. Global directory defaults are copied at creation;
changing global defaults does not change existing workspaces.

Directory settings are safe relative paths. Changing them does not move files.
Applications without overrides immediately use the new workspace defaults.
Validation and build state are recalculated on refresh. Future tenant settings
can be associated with workspace identity; credentials/tokens must remain in
protected user-local storage, never portable workspace JSON.

## Application settings

`inpakker.app.json`:

```json
{
  "schemaVersion": 1,
  "id": "dcb65432-1122-4344-8566-778899aabbcc",
  "name": "Mozilla Firefox",
  "setupFile": "Firefox Setup.exe"
}
```

`sourceDirectory` and `outputDirectory` optionally override workspace defaults.
`installCommand` and `uninstallCommand` are optional metadata, not packaging
commands. `setupFile` is relative to the effective source directory; source and
output are relative to the application root. UUIDs are generated during
creation; normal workflows do not require entering or displaying them.

`show --json` includes raw configuration and effective values with inheritance
flags. Validation, build, package lookup, fingerprints, diagnostics and the TUI
use the same effective-setting resolution.

## Typed edits

```powershell
inpakker config show
inpakker config show --workspace-settings
inpakker config set preferences.showToolOutput true
inpakker config set workspaceDefaults.outputDirectory packages
inpakker config set --workspace-settings name "Customer Production"
inpakker config set --workspace-settings sourceDirectory installer
```

Known global keys: `preferences.showToolOutput`,
`workspaceDefaults.applicationsDirectory`, `workspaceDefaults.sourceDirectory`,
`workspaceDefaults.outputDirectory`.
Known workspace keys: `name`, `applicationsDirectory`, `sourceDirectory`,
`outputDirectory`. Tools use `tools set`, `tools clear`, `tools detect` and
`tools install`. `--workspace-settings` selects the configuration scope;
`--workspace <name-or-path>` selects the workspace itself.

## Local build state

`state/workspaces/<workspace UUID>/build-cache.json` under Inpakker home uses
application UUID keys. Fingerprints include effective settings, source names
and content, and packaging tool identity. A matching record skips packaging
only while every recorded artifact exists. Failed builds do not update records.
`--force` rebuilds and saves state; `--no-cache` leaves state untouched.
