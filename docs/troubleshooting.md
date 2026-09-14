# Troubleshooting

Start with:

```powershell
inpakker doctor
```

For structured output:

```powershell
inpakker doctor --json
```

`doctor` checks the current workspace, application configuration and external-tool availability and is usually the fastest way to understand what is wrong.

---

## `inpakker` is not recognized

Open a new PowerShell window and try:

```powershell
inpakker --version
```

If the command is still unavailable, run the installer again:

```powershell
irm https://raw.githubusercontent.com/LickABrick/Inpakker/master/install.ps1 | iex
```

Inpakker normally installs for the current user and does not require administrator privileges.

---

## No workspace is selected

Open Inpakker:

```powershell
inpakker
```

and create or add a workspace from the Workspaces screen.

From the CLI, create one:

```powershell
inpakker workspace create D:\Intune\Customer-A --name "Customer A"
```

or register an existing workspace:

```powershell
inpakker workspace add D:\Intune\Customer-A
```

Then select it:

```powershell
inpakker workspace use "Customer A"
```

You can also target a workspace for one command:

```powershell
inpakker --workspace "Customer A" build --all
```

---

## Inpakker selected the wrong workspace

Check the workspace currently being used:

```powershell
inpakker workspace show
```

An explicitly selected workspace takes priority over automatic discovery.

Possible selectors include:

```text
--workspace
INPAKKER_WORKSPACE
workspace found in the current/parent directory
active registered workspace
```

Use:

```powershell
inpakker workspace list
```

to inspect registered workspaces.

Switch explicitly when needed:

```powershell
inpakker workspace use "Customer A"
```

---

## A registered workspace path is unavailable

The workspace remains registered when its drive, share or directory disappears.

Reconnect the original location if possible.

If the workspace has moved, relink it:

```powershell
inpakker workspace relink "Customer A" E:\Intune\Customer-A
```

The destination must be the same workspace identity.

Relinking an unrelated workspace is rejected.

---

## I want to remove a workspace from Inpakker

Remove its registration:

```powershell
inpakker workspace remove "Customer A" --yes
```

This does **not** delete:

* the workspace folder;
* application sources;
* packages;
* workspace configuration.

It only removes the registration from the current user's Inpakker configuration.

---

## Application name is ambiguous

If multiple applications have the same human-readable name, Inpakker asks for a more specific target.

Use the canonical relative application path shown in the error.

For example:

```powershell
inpakker build Browsers/mozilla-firefox
```

instead of:

```powershell
inpakker build "Mozilla Firefox"
```

when the name is ambiguous.

---

## Setup file is missing

Inspect the application:

```powershell
inpakker show "Mozilla Firefox"
```

or:

```powershell
inpakker show "Mozilla Firefox" --json
```

Check:

* effective source directory;
* configured setup filename;
* application root.

Place the installer under the effective source directory.

Example:

```text
apps/
└── Browsers/
    └── mozilla-firefox/
        └── source/
            └── Firefox Setup.exe
```

Changing source-directory settings does not move existing files automatically.

---

## Application is invalid

Run:

```powershell
inpakker validate "Mozilla Firefox"
```

For all applications:

```powershell
inpakker validate --all
```

Typical causes include:

* missing setup file;
* invalid source/output path;
* unsafe relative path;
* missing application configuration;
* malformed JSON;
* unsupported configuration schema;
* filesystem links inside packaging source.

Fix the reported problem and validate again.

---

## Source directory contains a symbolic link

Packaging source is expected to remain inside the application directory.

Replace symbolic links/junction-like paths with normal files or directories where required.

This prevents packaging input from unexpectedly escaping the application source tree.

---

# External tools

## Content Prep Tool is missing

The Microsoft Win32 Content Prep Tool is required to build `.intunewin` packages.

Try automatic detection:

```powershell
inpakker tools detect
```

Inspect tool configuration:

```powershell
inpakker doctor
```

If you already have the executable:

```powershell
inpakker tools set content-prep C:\Tools\IntuneWinAppUtil.exe
```

Or install the supported upstream tool through Inpakker:

```powershell
inpakker tools install content-prep --accept-license
```

You can also configure it from:

```text
Settings → External tools
```

Workspace and application management continues to work even when the packaging tool is unavailable.

---

## Decoder is missing

The package decoder is optional.

It is required only when unpacking `.intunewin` packages.

Configure an existing executable:

```powershell
inpakker tools set decoder C:\Tools\IntuneWinAppUtilDecoder.exe
```

or install the supported upstream decoder:

```powershell
inpakker tools install decoder --accept-license
```

Building applications does not require the decoder.

---

## Inpakker detected a tool but still uses the configured path

An explicitly configured tool path is preserved, even when it no longer works.

Inpakker does not silently replace it with another discovered executable.

Open:

```text
Settings → External tools
```

and choose the detected executable, or set the correct path manually:

```powershell
inpakker tools set content-prep C:\Correct\IntuneWinAppUtil.exe
```

This behavior prevents automatic discovery from unexpectedly changing a user's explicit configuration.

---

## Tool download fails

Check:

* internet access;
* GitHub/upstream availability;
* proxy/firewall restrictions;
* TLS inspection or endpoint security blocking the download.

Retry after connectivity is restored.

Do not bypass checksum/signature/security validation to force a failed tool or Inpakker update installation.

---

# Build problems

## Build says the application is already up to date

Inpakker uses local build fingerprints to avoid rebuilding unchanged applications.

To rebuild anyway:

```powershell
inpakker build "Mozilla Firefox" --force
```

To build without reading or updating cache state:

```powershell
inpakker build "Mozilla Firefox" --no-cache
```

If the state still looks wrong, inspect the application:

```powershell
inpakker show "Mozilla Firefox"
```

and verify that:

* source files are in the expected source directory;
* the setup filename is correct;
* output files still exist;
* the configured packaging tool is valid.

---

## Build cache error

Run:

```powershell
inpakker doctor
```

Build cache lives under the current user's Inpakker state directory.

If `INPAKKER_HOME` is set, verify that the location exists and is writable.

Do not point `INPAKKER_HOME` at a portable workspace.

---

## Packaging works manually but fails in Inpakker

Check:

```powershell
inpakker doctor
```

Then verify the configured tool path:

```powershell
inpakker tools detect
```

and inspect the application configuration:

```powershell
inpakker show "Application Name"
```

Pay attention to:

* setup filename;
* effective source directory;
* output directory;
* inaccessible files;
* filesystem links;
* Content Prep Tool output.

If **Show tool output** is disabled, packaging diagnostics are still retained for failed operations and can be opened from the result dialog.

---

# Unpacking

## No `.intunewin` package was found

Make sure the application has a built package in its effective output directory.

Build first if necessary:

```powershell
inpakker build "Mozilla Firefox"
```

Then:

```powershell
inpakker unpack "Mozilla Firefox"
```

If multiple packages exist, Inpakker may ask you to choose one.

---

## Unpack destination already exists

Choose a different destination:

```powershell
inpakker unpack C:\Packages\app.intunewin --destination C:\Decoded\App-Test
```

or deliberately replace the existing destination with:

```powershell
inpakker unpack C:\Packages\app.intunewin --destination C:\Decoded\App --force
```

Use `--force` only when overwriting the destination is intentional.

---

## Decoder output is rejected

Inpakker validates decoded archive paths before extraction.

Unsafe paths or filesystem links are rejected.

Do not disable this safety behavior.

If a package generated by another tool cannot be unpacked, confirm that the package and decoder are valid and retry with another destination.

---

# Folder opening

## Explorer does not open

Verify that the target directory exists.

Folder opening is supported in an interactive Windows desktop session.

For example:

```powershell
inpakker open
```

opens the workspace root.

```powershell
inpakker open "Mozilla Firefox"
```

opens the application directory.

```powershell
inpakker open "Mozilla Firefox" --output
```

opens the effective output directory.

Explorer is not expected to work on non-Windows development hosts.

---

# TUI problems

## Terminal is too small

The current minimum supported terminal size is:

```text
60 columns × 18 rows
```

Resize the terminal window.

If a larger terminal is unavailable, use the CLI instead.

---

## Colors or symbols are difficult to read

Enable accessible/plain presentation:

```powershell
$env:INPAKKER_ACCESSIBLE = "1"
inpakker
```

or:

```powershell
$env:ACCESSIBLE = "1"
inpakker
```

Inpakker uses text/symbol labels as well as color for important states.

---

## A background refresh seems stuck

Navigation should remain available while background refresh/tool/update checks run.

If the state does not update after the operation completes:

1. press the refresh action in the TUI;
2. reopen the workspace;
3. run:

```powershell
inpakker doctor
```

If the issue is reproducible, include the relevant non-sensitive diagnostics in a bug report.

---

## An operation is cancelling but has not closed yet

When `Ctrl+C` cancels an active build/unpack/installation operation, Inpakker waits for the worker process to acknowledge cancellation.

The TUI may show:

```text
Cancelling…
```

until cleanup completes.

This prevents a new conflicting operation from starting while the previous worker is still active.

---

## I cannot remember a shortcut

Press:

```text
?
```

inside the TUI.

The detailed interface guide is available at:

[TUI guide](tui.md)

Prefer contextual help rather than relying on remembered shortcuts.

---

# Updates

## Update check fails

Check internet connectivity to GitHub.

Run explicitly:

```powershell
inpakker update --check
```

Automatic update-check failures do not prevent normal Inpakker usage.

---

## Development build cannot update itself

Development builds identify themselves as:

```text
dev
```

and do not replace themselves through the self-updater.

Use a released build when testing the normal update workflow.

---

## Signature or checksum validation failed

Do not bypass the validation.

Download or update again after verifying connectivity and the official GitHub release.

If an official release repeatedly fails integrity validation, report it instead of forcing installation.

---

# Installation and uninstall

## Reinstall Inpakker

Run:

```powershell
irm https://raw.githubusercontent.com/LickABrick/Inpakker/master/install.ps1 | iex
```

This installs the current stable release for the user.

---

## Uninstall Inpakker

Run:

```powershell
irm https://raw.githubusercontent.com/LickABrick/Inpakker/master/uninstall.ps1 | iex
```

Workspace directories are preserved.

---

## Purge was refused

The uninstaller protects workspace data.

A purge can be refused when Inpakker's local state:

* overlaps a registered workspace;
* contains a filesystem link that makes safe deletion uncertain.

Preserve workspace data and remove unwanted local state manually only after verifying the affected paths.

---

# `INPAKKER_HOME`

`INPAKKER_HOME` redirects Inpakker's machine-specific data.

It is useful for:

* development;
* tests;
* isolated configurations.

Example:

```powershell
$env:INPAKKER_HOME = "C:\Temp\Inpakker-Test"
```

Do not point it at a workspace that contains application data.

Normal users generally do not need to set it.

---

# Reporting a problem

When opening a public issue, include:

* Inpakker version;
* Windows version;
* the command/action that failed;
* expected behavior;
* observed behavior;
* sanitized `inpakker doctor` output where useful;
* reproduction steps.

Do **not** include:

* credentials;
* tokens;
* tenant identifiers;
* proprietary packages;
* customer application source;
* sensitive logs or personal data.

Security issues should be reported privately according to:

[SECURITY.md](../SECURITY.md)
