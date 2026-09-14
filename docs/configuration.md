# Configuration

Inpakker separates **portable workspace configuration** from **machine-specific user state**.

The basic rule is:

> Anything needed to move a workspace between machines belongs with the workspace. Anything specific to one computer or user stays in the Inpakker user directory.

---

## Configuration locations

### User configuration

On Windows, machine-specific Inpakker data lives under:

```text
%LOCALAPPDATA%\Inpakker
```

For development and testing, the location can be overridden with:

```text
INPAKKER_HOME
```

Typical contents include:

```text
Inpakker/
├── config.json
├── tools/
├── state/
│   └── workspaces/
│       └── <workspace UUID>/
│           └── build-cache.json
└── update/
    └── state.json
```

This data is not intended to travel with a workspace.

---

## Global user settings

The global configuration file is:

```text
%LOCALAPPDATA%\Inpakker\config.json
```

Example:

```json
{
  "schemaVersion": 1,
  "tools": {
    "contentPrepTool": {
      "path": "C:\\Tools\\IntuneWinAppUtil.exe"
    },
    "decoder": {
      "path": ""
    }
  },
  "preferences": {
    "showToolOutput": false
  },
  "workspaceDefaults": {
    "applicationsDirectory": "apps",
    "sourceDirectory": "source",
    "outputDirectory": "output"
  },
  "workspaces": [],
  "activeWorkspaceId": ""
}
```

Global user configuration can contain:

* external tool paths;
* user preferences;
* defaults for newly created workspaces;
* registered workspace locations;
* the active workspace.

These values are specific to the current user/machine.

---

## Workspace defaults

Global workspace defaults are used when creating a new workspace.

Default values are typically:

```text
Applications directory    apps
Source directory          source
Output directory          output
```

Changing a global default affects **future workspaces**.

It does not silently change existing workspaces.

Example:

```powershell
inpakker config set workspaceDefaults.outputDirectory packages
```

A workspace created afterward can use:

```text
packages
```

as its output directory.

Existing workspaces keep their own configuration.

---

## Workspace registrations

Inpakker can register multiple independent workspaces.

Registrations are stored in global user configuration and contain information such as:

* workspace identity;
* local path;
* last-opened time.

The actual workspace name is stored in the workspace itself.

If a registered path becomes unavailable, the registration is retained so it can be reconnected or relinked later.

Removing a workspace registration does **not** delete the workspace files.

---

# Workspace configuration

Each workspace contains:

```text
inpakker.workspace.json
```

Example:

```json
{
  "schemaVersion": 1,
  "id": "f13cb18e-1122-4344-8566-778899aabbcc",
  "name": "Customer A",
  "applicationsDirectory": "apps",
  "sourceDirectory": "source",
  "outputDirectory": "output"
}
```

The workspace configuration is portable and should be safe to keep in version control.

---

## Workspace identity

Each workspace has a generated UUID:

```json
"id": "f13cb18e-1122-4344-8566-778899aabbcc"
```

The UUID identifies the workspace independently of:

* its display name;
* its filesystem location.

This allows a workspace to be renamed or moved without becoming a different workspace.

Do not manually reuse one workspace UUID for multiple unrelated workspaces.

---

## Workspace name

Example:

```json
"name": "Customer A"
```

The name is the human-readable workspace name shown by Inpakker.

It may contain spaces.

The local registration path can change without changing the workspace name or identity.

---

## Applications directory

Example:

```json
"applicationsDirectory": "apps"
```

This determines where applications live relative to the workspace root.

Example:

```text
Customer-A/
├── inpakker.workspace.json
└── apps/
    ├── Browsers/
    └── Utilities/
```

The path must be a safe relative path.

---

## Source directory

Example:

```json
"sourceDirectory": "source"
```

The source directory contains files used to build an application.

Example:

```text
apps/Browsers/mozilla-firefox/
├── inpakker.app.json
└── source/
    ├── Firefox Setup.exe
    └── Install.ps1
```

---

## Output directory

Example:

```json
"outputDirectory": "output"
```

The output directory contains generated package artifacts.

Example:

```text
apps/Browsers/mozilla-firefox/
└── output/
    └── Firefox Setup.intunewin
```

---

## Changing workspace directories

Changing a workspace directory setting changes where Inpakker expects files to be.

It does **not** move existing files automatically.

For example, changing:

```text
source
```

to:

```text
installer
```

does not rename or move the existing `source` directory.

Move files manually when required.

Applications that inherit the workspace setting will use the new value immediately.

---

# Application configuration

Each application contains:

```text
inpakker.app.json
```

Example:

```json
{
  "schemaVersion": 1,
  "id": "dcb65432-1122-4344-8566-778899aabbcc",
  "name": "Mozilla Firefox",
  "setupFile": "Firefox Setup.exe"
}
```

---

## Application identity

Like workspaces, applications use generated UUIDs.

Example:

```json
"id": "dcb65432-1122-4344-8566-778899aabbcc"
```

The UUID remains stable when an application directory or display name changes.

Normal users generally do not need to edit or reference UUIDs directly.

---

## Application name

Example:

```json
"name": "Mozilla Firefox"
```

This is the human-readable application name.

Application names can contain spaces.

Applications may be placed inside nested groups such as:

```text
Browsers/Mozilla-Firefox
Microsoft/Office/Microsoft-365-Apps
Utilities/7-Zip
```

---

## Setup file

Example:

```json
"setupFile": "Firefox Setup.exe"
```

The setup file is relative to the application's effective source directory.

For example:

```text
apps/Browsers/mozilla-firefox/source/Firefox Setup.exe
```

---

## Source and output overrides

Applications normally inherit their source and output directory names from the workspace.

An application may override them:

```json
{
  "schemaVersion": 1,
  "id": "dcb65432-1122-4344-8566-778899aabbcc",
  "name": "Mozilla Firefox",
  "setupFile": "Firefox Setup.exe",
  "sourceDirectory": "installer",
  "outputDirectory": "package"
}
```

Only that application uses the override.

Other applications continue using the workspace defaults.

---

## Install and uninstall commands

Applications may optionally contain:

```json
{
  "installCommand": "powershell.exe -ExecutionPolicy Bypass -File Install.ps1",
  "uninstallCommand": "powershell.exe -ExecutionPolicy Bypass -File Uninstall.ps1"
}
```

These values are application metadata.

Normal packaging does **not** execute them.

They can describe how the resulting application is expected to install or uninstall and may be used by explicit future testing or publishing workflows.

Do not assume that simply defining these commands performs deployment validation.

---

# Effective application settings

An application's effective configuration combines:

* workspace defaults;
* optional application overrides.

For example:

```text
Workspace source directory      source
Application source override     <none>
Effective source directory      source
```

or:

```text
Workspace source directory      source
Application source override     installer
Effective source directory      installer
```

Validation, packaging, package lookup and the TUI use the same effective values.

Inspect effective settings with:

```powershell
inpakker show "Mozilla Firefox" --json
```

---

# Editing configuration

## Show global configuration

```powershell
inpakker config show
```

## Show workspace configuration

```powershell
inpakker config show --workspace-settings
```

## Change a preference

```powershell
inpakker config set preferences.showToolOutput true
```

## Change a new-workspace default

```powershell
inpakker config set workspaceDefaults.outputDirectory packages
```

## Change the current workspace name

```powershell
inpakker config set --workspace-settings name "Customer Production"
```

## Change a workspace source directory

```powershell
inpakker config set --workspace-settings sourceDirectory installer
```

The TUI also provides typed editors for supported settings.

---

# Supported global keys

Current global typed settings include:

```text
preferences.showToolOutput
workspaceDefaults.applicationsDirectory
workspaceDefaults.sourceDirectory
workspaceDefaults.outputDirectory
```

External tools are managed through the dedicated tool commands rather than raw config editing.

Examples:

```powershell
inpakker tools detect
inpakker tools set content-prep C:\Tools\IntuneWinAppUtil.exe
inpakker tools clear decoder
```

---

# Supported workspace keys

Current workspace settings include:

```text
name
applicationsDirectory
sourceDirectory
outputDirectory
```

Use:

```powershell
--workspace-settings
```

to edit workspace configuration.

Use:

```powershell
--workspace <name-or-path>
```

to select which workspace a command should operate on.

These flags have different purposes.

---

# Local build state

Build cache is stored outside the portable workspace under:

```text
%LOCALAPPDATA%\Inpakker\state\workspaces\<workspace UUID>\build-cache.json
```

The cache is local state and should not be committed with a workspace.

A cached build can be skipped only when the relevant inputs still match and the expected output package still exists.

---

## Normal build

```powershell
inpakker build "Mozilla Firefox"
```

A matching unchanged application can be skipped.

---

## Force rebuild

```powershell
inpakker build "Mozilla Firefox" --force
```

This rebuilds the application even when the cache says it is unchanged.

Successful state is recorded afterward.

---

## Build without cache

```powershell
inpakker build "Mozilla Firefox" --no-cache
```

This ignores existing cache state and does not update it.

---

# Portable versus local data

A useful rule of thumb:

### Keep with the workspace

```text
inpakker.workspace.json
inpakker.app.json
application source files
custom install/uninstall scripts
other packaging inputs
```

### Keep local to the machine

```text
external tool paths
managed tools
workspace registrations
active workspace
build cache
update state
credentials or tokens
```

Do not place credentials, tokens or other secrets in portable workspace/application JSON.

---

# Schema versions

Persisted Inpakker configuration files use:

```json
"schemaVersion": 1
```

Unsupported schema versions are rejected instead of being interpreted silently.

Do not manually change schema versions.

When a future release introduces a new schema, its release notes and documentation will describe any compatibility requirements.

---

# Paths

Workspace and application directory settings must remain safe relative paths.

Avoid:

```text
absolute paths
parent traversal such as ..
filesystem links that escape application/workspace boundaries
Windows reserved filenames
```

Use normal workspace-relative directories instead.

---

# Version control

Portable workspaces are designed to work well with Git.

A typical repository might contain:

```text
Customer-A/
├── inpakker.workspace.json
└── apps/
    ├── Browsers/
    │   └── mozilla-firefox/
    │       ├── inpakker.app.json
    │       └── source/
    │           ├── Install.ps1
    │           └── Uninstall.ps1
    └── Utilities/
```

Generated `.intunewin` packages and vendor installer binaries are commonly excluded from Git depending on your packaging workflow.

Do not commit credentials or secrets.

---

# More help

For TUI usage:

[Terminal interface](tui.md)

For common problems:

[Troubleshooting](troubleshooting.md)

For planned future functionality:

[Roadmap](../ROADMAP.md)
