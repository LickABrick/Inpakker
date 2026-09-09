# Troubleshooting

Run `inpakker doctor` first. It identifies missing configuration, directories,
external tools, and invalid applications without changing the workspace.

## Command not found

Run the executable using its full path, for example:

```powershell
C:\Tools\Inpakker\inpakker.exe --version
```

If that works, add its directory—not the executable itself—to your user `PATH`,
then open a new terminal.

## Packaging utility is missing

Download Microsoft's Win32 Content Prep Tool separately and update
`intunewinapputil` in `inpakker.config.json` with the full path to
`IntuneWinAppUtil.exe`.

## An application is invalid

Run:

```powershell
inpakker validate myapp
inpakker show myapp
```

Confirm that `source` exists and that `setupFile` names a file inside that
directory. Configuration paths may not be absolute or escape their application
or workspace root.

## A package is unexpectedly up to date

Use `inpakker build myapp --force` to deliberately rebuild it. Inpakker normally
rebuilds whenever application configuration, source contents, or the configured
packaging utility changes, and whenever a recorded output is missing.

## Terminal display problems

Use a current Windows Terminal or PowerShell terminal. Redirected output does
not contain animation or forced ANSI styling. Set `NO_COLOR=1` to disable color,
or `INPAKKER_ACCESSIBLE=1` for screen-reader-friendly guided forms.

## Update checks fail

Explicitly run `inpakker update --check`. Inpakker uses the system proxy settings
and requires HTTPS access to `api.github.com` and GitHub release downloads.
Ordinary daily checks fail silently so packaging work is never blocked by an
offline network.

An update cannot replace an executable in a directory where the current user
does not have write permission. Move Inpakker to a user-writable tools directory
or deliberately run the update from an appropriately elevated terminal. It does
not request elevation automatically.

## Getting more detail

When opening a bug report, include:

- `inpakker --version` output;
- Windows version and terminal application;
- the command and complete error message;
- a minimized, redacted configuration if relevant.

Never attach proprietary `.intunewin` packages, application source, credentials,
tenant identifiers, or sensitive logs to a public issue.
