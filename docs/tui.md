# Terminal interface

`inpakker` starts the TUI in an interactive terminal; redirected invocation
prints help. `inpakker tui` explicitly starts it. The shell displays decorative
📦 INPAKKER branding, workspace name, path, inventory count and background
activity. Version and update controls live in About (`i`).

## Navigation

| Key | Action |
| --- | --- |
| `w` | Workspaces: switch, create, add, relink, remove |
| `s` | Global settings, workspace settings, external tools |
| `i` | About, version, license, updates |
| `?` | Full shortcut guide |
| `Backspace` / `Esc` | Back; focused inputs and dialogs take precedence |
| `q` | Quit |
| `r` | Background refresh on Applications/details |
| `o` | Open the relevant folder in Explorer |

Workspaces is also the switcher. It sorts active first, then recently opened,
then name. `/` searches name/path; finish searching with Enter or Esc, then
Enter switches, `o` opens the highlighted workspace without switching, `n`
creates, `a` adds, `e` edits its name, `r` relinks and `x` removes registration.
Unavailable registrations remain visible. Removing never deletes workspace files.

With no workspace, creation and add-existing actions are shown instead of an
empty Applications table. Tools may be configured later.

## Applications

The table uses the available height, with separate validation, build and package
columns on wide terminals, combined state at medium sizes and application/state
on narrow screens. Build labels are ✓ Up to date, • Needs build, ○ Not built,
— Unavailable and ! Unknown.

`/` searches; Enter opens details. `b` builds; `B` offers Build, Rebuild and Build
without cache. `v` validates; `u` unpacks. Ctrl+B/Ctrl+V/Ctrl+U act on all apps.
Press `a` on Applications or details to open a visible action menu for the selected
application: Build, Build options / rebuild, Validate, Unpack, Open application
folder and Diagnostics. Arrows select, Enter runs the action, and Esc returns.
Details show effective source/output values and whether they are inherited.

`n` opens one Create application form: name, generated directory name, group,
setup file and inherited defaults. Editing the directory name stops automatic
slug updates. The optional Group field accepts an existing or new relative path,
including nested groups; Ctrl+E completes a suggested existing group. Leave it
empty for no group. In the Setup file field, type a relative filename to add
later, or press F2 to select an existing installer (including MSI, EXE and scripts).
The selected file will be copied into the new source folder; companion files
are not copied automatically. Editing the setup filename clears the copy selection.

**Review application** shows the resolved application, source, setup and output
paths before writing anything. Arrows and PgUp/PgDn scroll the preview. Enter
creates the application; Esc returns to editing with the draft intact. Ctrl+C
cancels the review. A failed creation preserves the draft for correction or retry.
The CLI equivalent is:

```powershell
inpakker new "Mozilla Firefox" --group Browsers --setup-from "C:\Downloads\Firefox Setup.exe" --no-input
```

`--setup-from` defaults the setup filename to the selected file's name. Combine
it with `--setup-file` to choose a different safe relative destination within the
source folder. Existing application directories are never overwritten.

## Forms and settings

Up/down and Tab/Shift+Tab navigate text fields; left/right move the cursor and
Backspace deletes. Select controls use arrows to choose an option and Enter to
advance. Global shortcuts do not trigger while typing. Workspace/tool path
forms and application setup selection offer F2 browsing with right-arrow directory
navigation and Enter selection;
manual paths remain available.

Settings uses a property list with focused edits. Directory changes explain that
files will not be moved. Boolean settings use On/Off controls. Directory edits
are validated before saving; a failed save keeps the entered value in the form
so it can be corrected or retried. To download tools, press `s`, select an **External tools**
row and press Enter. Choose **Download from official source**, then accept the
upstream license terms to start downloading. The same menu offers **Choose
existing executable** for a tool already on disk. No workspace is required.

`d` detects tools. With a tool selected, `I` opens the download confirmation
directly, `a` accepts a detected candidate, and `x` clears its configured path
while preserving the executable.
The decoder is optional and required only for unpacking. If a build or unpack
cannot use its required tool, the setup menu opens directly over the current
page. Choose download or an existing executable, then retry the operation.
Downloads still require license acceptance; Esc leaves the tool unchanged.

## Background work and progress

Refresh, workspace loading, tool detection and update checks run asynchronously.
Navigation and folder opening stay available. Inventory results carry workspace
identity and request generation, so a previous workspace or older refresh cannot
replace current data. Foreground builds, unpacking and installations block
conflicting actions and support Ctrl+C cancellation.
Cancellation displays “Cancelling…” until the worker has stopped. Completed
build, validation and unpack results remain available afterward.

Progress shows the current item (for example “4 of 4”), phase and completed count.
Completion replaces progress with results inside a dialog, keeping the current
page behind it. This applies to single and batch builds, validation and unpacking,
as well as update and tool installation results. Success, failure, warning and
active status text use distinct colors alongside their text/symbol labels.

In batch results, Up/down selects an application and PgUp/PgDn scrolls the full
details. Enter opens the selected application's page when available; Esc closes
the dialog and returns to the original page. In other result dialogs, arrows and
PgUp/PgDn scroll long messages. `l` opens available build tool output in the dialog;
Enter or Esc returns to the result. Saving/scaffolding dialogs remain open until
their writes finish.

Press `a` in a result dialog for its available actions: view tool output, open
output folder, retry failures, and open application details. Actions appear when
the corresponding log, output, failed targets or application is available. Esc
returns to the same result and selected row. Existing shortcuts remain available.

`o` opens the selected result's output folder (or unpack destination). `r` retries
failed applications only, keeping the original build options or selected unpack
package. Retry uses the operation's workspace and application identities, so it
cannot run against a different selected application or workspace. Missing targets
must be refreshed before retrying. The CLI equivalent for opening package output
is `inpakker open "Mozilla Firefox" --output`; retry CLI operations by specifying
the failed targets again.

`L` reopens the last operation result, including its log, during the current TUI
session. Workspace results are available only in their originating workspace;
tool and update results are global. This history is kept in memory and is not
saved when Inpakker exits. Packaging logs retain the last 256 KiB even when
“Show tool output” is disabled, with a notice when older output was omitted.

Tool installation reports source lookup, downloading, verification and installation.
Downloads show received bytes and a progress bar when the upstream supplies a
total size; unknown sizes show received bytes without an estimated percentage.

## Folder context

Applications/details open the application root, not source or output. Workspaces
opens the highlighted workspace without selecting it. Diagnostics and workspace
settings open the active workspace root. About, global settings and no-workspace
screens have no folder action. Explorer starts asynchronously through direct
arguments; missing folders and launch failures produce friendly errors.

## About and accessibility

About shows current version, MIT license, repository and update status. `c` checks
for updates in the background; `u` installs an available update after confirmation.
Development builds cannot install updates. Available updates produce a subtle
shell hint pointing to About.
`INPAKKER_NO_UPDATE_CHECK=1` disables automatic checks only; About's explicit
check and install actions remain available.

Set `INPAKKER_ACCESSIBLE=1` or `ACCESSIBLE=1` for plain branding. CLI forms also
support Huh accessible mode. State never relies on color. The minimum terminal is
60 columns by 18 rows; smaller terminals show an explanatory fallback.
