# Troubleshooting

Start with `inpakker doctor` or `inpakker doctor --json`. Diagnostics distinguish
workspace configuration/applications and global tools/settings.

- **Command not found:** open a new PowerShell session or run the installer again.
  The per-user executable normally lives under `%LOCALAPPDATA%\Programs\Inpakker`.
- **No workspace selected:** run `workspace create <directory> --name "Name"`,
  `workspace add <directory>` and `workspace use "Name"`, or set `--workspace`.
- **Unexpected workspace:** an explicit selector, then `INPAKKER_WORKSPACE`, then
  current/parent discovery take precedence over the active registered workspace.
- **Path unavailable:** reconnect the drive/share or use `workspace relink`.
  Relink must point to the same workspace UUID. Missing paths remain registered.
- **Ambiguous application:** use the canonical relative target listed in the
  error, for example `Browsers/mozilla-firefox`.
- **Content Prep Tool missing:** use `tools detect`, `tools set content-prep <path>`
  or `tools install content-prep --accept-license`. Review upstream license terms
  and prerequisites. Missing tools do not prevent workspace/application management.
- **Decoder missing:** use `tools set decoder <path>` or
  `tools install decoder --accept-license`. This tool is optional.
- **Detected replacement not selected:** explicit configured paths, even invalid
  ones, are preserved. Use `tools set` or Settings → tool → `a` to accept the candidate.
- **Setup file missing:** place it under the effective Source directory shown by
  `show`. Changing workspace defaults does not move existing source files.
- **Source contains a symbolic link:** use regular files and directories in the
  Source directory. Packaging rejects links so inputs remain inside the application.
- **Unexpected build state:** use `show`, validate paths, and check the packaging
  executable. `--force` rebuilds and records state; `--no-cache` ignores and
  preserves state. Cache errors include the affected user-local cache path.
- **Unpack destination exists:** choose another `--destination`, or deliberately
  use `--force`. Unsafe archive paths and links are rejected.
- **Folder cannot open:** verify the selected directory still exists and run in
  an interactive Windows desktop session. No Explorer process runs on Linux.
- **Update/download failed:** verify network access to GitHub and upstream raw
  content. Signature/checksum failures prevent installation. Do not bypass them.
  Development builds report `dev` and cannot replace themselves.
- **Purge refused:** global state overlaps a workspace or contains a filesystem
  link. Preserve workspace data while removing unwanted state manually.
- **Terminal too small:** resize to at least 60×18. Use CLI commands or set
  `INPAKKER_ACCESSIBLE=1` for accessible CLI forms and plain TUI branding.

`INPAKKER_HOME` redirects all global settings, managed tools, build state and
update cache. It is useful for isolated development and testing. Do not point it
at a workspace containing application data. Never include credentials, source
packages or customer details in public issue reports.
