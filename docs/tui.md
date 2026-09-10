# Terminal interface

Run `inpakker` in an interactive terminal from a workspace directory. If the
directory is not initialized, Inpakker opens the staged setup wizard. You can
also start the interface explicitly with `inpakker tui`.

The interface uses one persistent shell: the current Inpakker version and
available update appear at the top, the active workspace is shown below it,
and the footer presents only the most relevant actions. Press `?` anywhere on
a normal page for the complete shortcut guide.

## Applications dashboard

The dashboard keeps three concepts separate:

- Validation reports whether the application config, source directory, and
  setup file are usable.
- Build reports whether the deterministic build cache and recorded artifact
  are current, need rebuilding, have never been built, or cannot be inspected.
- Package reports how many `.intunewin` files currently exist.

The table adapts automatically at wide, medium, and narrow terminal widths. It
scrolls when the workspace has more applications than fit on screen.

Press `/` for inline fuzzy search across names, display names, groups, paths,
and states. While search is active, normal characters and Backspace edit the
query, arrow keys move through results, Enter opens the selected app, and Esc
clears the search.

## Keys

| Key | Action |
| --- | --- |
| `Up` / `k`, `Down` / `j` | Move or scroll |
| `Enter` | Open or confirm |
| `Backspace` | Return to the previous page when no input owns it |
| `Esc` | Back, close, clear, or cancel |
| `/` | Search applications |
| `n` | New application wizard |
| `b` | Smart build selected application |
| `B` | Selected-app build options: smart, forced, or no cache |
| `Ctrl+B` | Build all applications |
| `v`, `Ctrl+V` | Validate selected or all applications |
| `u`, `Ctrl+U` | Unpack selected or all applications |
| `d` | Workspace diagnostics |
| `r` | Refresh disk-backed workspace state |
| `U` | Review and install an available verified update |
| `l` | Open captured packaging output when offered |
| `?` | Keyboard shortcut dialog |
| `q` | Quit from a normal page |
| `Ctrl+C` | Cancel active work; otherwise quit |

Text inputs and forms receive key presses before global shortcuts, so typing
`q`, `/`, `b`, or Backspace cannot accidentally trigger navigation or actions.

## Workflows

The setup and new-application dialogs are staged so only the current decisions
are visible. Each finishes with a review step. Creating an app refreshes the
dashboard and keeps the new application selected.

A normal `b` build immediately uses the existing smart cache. `B` exposes force
and no-cache modes without adding confirmation to everyday builds. Batch
operations finish independently processable applications and open a results
page when there is more than one result. Packaging-tool output is kept in a
separate scrollable log instead of being dumped into a dialog.

Unpack is clearly marked unavailable when no decoder or package is present. If
an app has multiple packages, `u` opens a package-selection dialog. Arbitrary
package paths remain available through `inpakker unpack <path>`.

Workspace Diagnostics maps the same checks as `inpakker doctor` into a
structured health page. `r` reruns the checks or reloads application config,
validation, package, and build state from disk, depending on the current page.

## Terminal size and accessibility

Inpakker requires at least 60 columns by 18 rows for the full interface. A
clear fallback is shown below that size, and resizing recovers automatically.
Colors adapt to light and dark backgrounds, while symbols and text communicate
every important state.

For screen-reader-friendly Huh prompts, use the CLI equivalents with
`INPAKKER_ACCESSIBLE=1` or `ACCESSIBLE=1`, for example:

```powershell
$env:INPAKKER_ACCESSIBLE = "1"
inpakker setup
inpakker new
```

Every TUI operation has a noninteractive command/flag equivalent for scripting
and assistive workflows.
