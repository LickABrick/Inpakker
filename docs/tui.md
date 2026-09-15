# Terminal interface

`inpakker` starts the TUI in an interactive terminal; redirected invocation
prints help. `inpakker tui` explicitly starts it. The shell displays decorative
📦 INPAKKER branding, workspace name, path, inventory count and background
activity. Version and update controls live in About (`i`).

## Navigation and discovery

Applications, Workspaces, Settings and About are destinations. Switching between
them replaces the current destination; Esc returns to Applications. Within an
application or diagnostics flow, Esc/Backspace returns to the parent. Focused
inputs keep Backspace for editing.

Use arrows and Enter to select and open items. Optional `j`/`k` aliases work
outside text inputs. Press `?` for the current page or action/result dialog's
bindings. The footer shows at most seven immediate actions from those same bindings; use
`?` for complete contextual help and `:` for command discovery.

Press `:` for the fuzzy command palette. Type a few letters, use arrows to
select, Enter to run, or Esc to close. It includes destinations and available
application, workspace, tool and update operations. Commands that need a selected
application stay scoped to that application. The palette also offers **Create
workspace** and **Add workspace**, including from other destinations.

`/` focuses filtering on Applications, Workspaces, Settings and workspace
diagnostics. Enter operates on the selected result and leaves text editing.
The query stays visible until cleared. Esc clears the filter and restores the
full list before navigating back. Letters such as `j`, `s`, `:` and `?` are
ordinary text while an input owns focus. No-match states explain how to clear
the query.

Workspaces sorts active first, then recently opened, then name. Active and
unavailable registrations have explicit text states; paths occupy a separate
line. Wider terminals show the selected workspace's path and status in a
preview. Enter switches to a ready workspace. `a` opens its available actions:
switch, open folder, rename, relink, or remove registration. Removal never
deletes workspace files. Create a workspace with `n`, or find creation and
add-existing operations through `:`. PgUp/PgDn moves through long lists.

With no workspace, creation and add-existing actions are shown instead of an
empty Applications list. Tools may be configured later.

## Applications

The list uses the available height and adds a concise preview on terminals around
100 columns and wider. The preview shows validation, build state, path, setup,
commands and package information. Narrow terminals keep a single list; duplicate
application names show relative paths and inventory numbers to remain distinguishable. Build labels are ✓ Up to date,
• Needs build, ○ Not built, — Unavailable and ! Unknown.

Enter opens details with the human-readable name, canonical relative path,
effective source/output, setup file, and install/uninstall command metadata.
Those commands are displayed, never executed by packaging.

Press `a` for selected-application actions: Build, Build options, Validate,
Unpack, Open application folder and Workspace diagnostics. Unavailable build
and unpack actions show a reason. Direct `b`/`u` shortcuts retain the existing
tool-setup recovery flow. `B` offers these modes:

- **Build** packages when inputs changed; reads and saves build state.
- **Rebuild** packages again even when unchanged and updates build state.
- **Build without cache** neither reads nor saves build state.

Ctrl+B / `V` / Ctrl+U operate on the complete application inventory, including
applications outside the current filter. Exact contextual shortcuts are listed
in `?`. Lowercase `v` validates only the selected application.

Workspace diagnostics has selectable checks. Wide terminals show the selected
check's details beside the list; Enter opens scrollable details at any size.
Failures include guidance for inspecting configuration or setting up tools.
The scriptable `inpakker doctor` command is unchanged.

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

Dialogs size to their content within the available terminal height. Small edits
and confirmations stay compact; longer forms scroll to keep the active field visible.

Up/down and Tab/Shift+Tab navigate text fields; left/right move the cursor and
Backspace deletes. Empty or invalid text fields do not block navigation.
Validation runs only when you confirm the final Save, Create or Review action.
An error returns focus to the first field needing attention and preserves the
rest of the form; you can still move freely between fields afterward. Submit
actions use the same button style as update confirmations.
Select controls use arrows to choose an option and Enter to advance. Global shortcuts do not trigger while typing. Workspace/tool path
forms and application setup selection offer F2 browsing with right-arrow directory
navigation and Enter selection;
manual paths remain available.

Settings groups focused edits under **New workspace defaults**, **Preferences**,
**External tools**, and **Current workspace**. New workspace defaults seed future
workspaces; changing them does not update existing workspaces. Boolean values
show On/Off. Tool rows show Ready, Not configured, Configured path unavailable,
Candidate detected, or Checking while their status is being inspected. Directory changes explain that
files will not be moved. Boolean settings use On/Off controls. Directory edits
are validated before saving; a failed save keeps the entered value in the form
so it can be corrected or retried. To download tools, press `s`, select an **External tools**
row and press Enter. Choose **Download from official source**, then accept the
upstream license terms to start downloading. The same menu offers **Choose
existing executable** for a tool already on disk. No workspace is required.

`d` detects tools. With a tool selected, Enter or `a` opens its setup actions,
including accepting a detected candidate or clearing configuration when available.
Clearing configuration preserves the executable.
The decoder is optional and required only for unpacking. If a build or unpack
cannot use its required tool, the setup menu opens directly over the current
page. Choose download or an existing executable, then retry the operation.
Downloads still require license acceptance; Esc leaves the tool unchanged.

## Background work and progress

Refresh, workspace loading, tool detection and update checks run asynchronously.
The header aggregates overlapping work into one background activity indicator.
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
details. In both single and batch application results, Enter opens the result
application's details when it is still available. Esc or Backspace closes the
dialog and returns to the original page. Generic messages without an application
target close with Esc or Backspace; Enter does not silently dismiss them.
Arrows and PgUp/PgDn scroll long messages. `l` opens available build tool output;
Enter, Esc or Backspace returns to the result. Saving/scaffolding dialogs remain
open until their writes finish.

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

Long detail, help, review and result views show a scroll percentage when more
content is available. Arrows and PgUp/PgDn scroll these views.

## Folder context

Applications/details open the application root, not source or output. Workspaces
opens the highlighted workspace without selecting it. Diagnostics and workspace
settings open the active workspace root. About, new workspace defaults and no-workspace
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
