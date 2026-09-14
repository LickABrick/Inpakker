# 🗺️ Inpakker Roadmap

Inpakker started as a simple way to organize and build Microsoft Intune Win32 applications.

The longer-term goal is broader:

> **Make Inpakker a fast, local-first development environment for creating, testing, maintaining and eventually publishing Intune Win32 applications.**

This roadmap describes the direction of the project rather than promising release dates.

Priorities may change as features are designed, tested and used in real environments.

---

## ✅ Today

Inpakker already provides the foundation:

* Multiple independent workspaces
* Nested application groups
* Application scaffolding
* Source and output directory inheritance
* Application validation
* `.intunewin` packaging
* Smart/incremental builds
* Forced and no-cache builds
* Package unpacking
* External tool management
* Application and workspace diagnostics
* CLI automation
* Interactive TUI
* Built-in update checking
* Cancellable operations and reusable results

The next features build on this workflow instead of replacing it.

---

# 🎨 1. TUI & UX modernization

**Status: Implemented — v0.6.0**

The terminal interface now provides reusable foundations for larger workflows.

The goal is to keep Inpakker easy for someone using arrow keys and Enter while making it very fast for experienced terminal users.

Implemented improvements include:

* Context-aware shortcuts and help
* Consistent `/` filtering throughout the TUI
* Global fuzzy command palette with `:`
* Improved application list with a preview pane on larger terminals
* Improved workspace management
* Cleaner Settings layout
* Better diagnostics detail views
* Contextual action menus
* Optional `j/k` list navigation alongside arrow keys
* Cleaner background activity indicators
* More consistent loading, empty and error states
* Better responsive layouts for different terminal sizes

See [the TUI guide](docs/tui.md) for the implemented workflows. Palette searches
remain synchronous because the in-memory command inventory is small. Optional
`h/l` aliases and a background-task detail view are not needed for these workflows.

---

# 🧪 2. Windows Sandbox application testing

**Status: Planned — before WinGet**

Building a package successfully does not necessarily mean the application installs correctly.

Inpakker should be able to launch a clean Windows Sandbox and test the actual application before it reaches Intune.

The first implementation is planned around **built `.intunewin` packages**.

### Capability detection

Inpakker should automatically determine whether Windows Sandbox testing is available.

Possible states include:

```text
✓ Ready
! Windows Sandbox feature disabled
! Restart required
X Unsupported Windows edition
X Virtualization unavailable
! Legacy Sandbox only
X Sandbox unavailable
```

Where supported, Inpakker should detect the modern Windows Sandbox automation capabilities as well.

### Package testing

A typical workflow could become:

```text
Application
    ↓
Validate
    ↓
Build .intunewin
    ↓
Test in Windows Sandbox
    ↓
Install as SYSTEM
    ↓
Collect results
```

Inpakker already knows the application's install command, so testing should use the actual application configuration instead of guessing how the package should be started.

### Test results

Initial results could include:

```text
Sandbox test · Mozilla Firefox

✓ Sandbox started
✓ Package prepared
✓ Install command completed

Exit code       0
Duration        12.8s
Context         SYSTEM

Result          ✓ Passed
```

Later testing can expand into a complete lifecycle:

```text
INSTALL
✓ Exit code 0

DETECTION
✓ Application detected

UNINSTALL
✓ Exit code 0

POST-DETECTION
✓ Application no longer detected
```

### Safe isolation

Application/package content should be exposed to the Sandbox read-only wherever possible.

Test results should be written to a separate temporary Inpakker location so a broken or malicious installer cannot modify the real workspace.

### Test state

A successful Sandbox test should be associated with the **exact application/build fingerprint**.

For example:

```text
✓ Sandbox tested
```

After changing the source or application configuration:

```text
○ Not tested since changes
```

Sandbox state should remain local build/test state rather than permanent portable application configuration.

A Sandbox test means:

> This package successfully ran in a clean Windows Sandbox environment.

It does **not** claim to fully reproduce Microsoft Intune, the Intune Management Extension, tenant configuration or customer policies.

---

# 📦 3. WinGet application source

**Status: Planned — after Sandbox testing**

WinGet can provide Inpakker with a large existing application catalog without requiring Inpakker to maintain its own.

The goal is **not** to make managed devices run `winget install`.

Instead, WinGet becomes a package source during application creation.

Conceptually:

```text
Search WinGet
     ↓
Select application
     ↓
Analyze package metadata
     ↓
Download exact installer
     ↓
Generate Inpakker application
     ↓
Build normal .intunewin
```

The resulting Intune application remains self-contained.

### WinGet availability detection

Inpakker should detect:

* WinGet availability
* WinGet version
* Community source availability
* Whether the installation is suitable for Inpakker automation

### Search

From the TUI:

```text
WinGet catalog

Search: firefox

Mozilla Firefox
Mozilla Firefox ESR
Firefox Developer Edition
```

Selecting a package should show available installers and relevant metadata.

### Packaging confidence

WinGet applications should receive an explainable **Packaging confidence** score.

This does not mean the application is safe or trusted.

It means:

> How confidently can Inpakker automatically create a reliable Intune package from the available metadata?

Possible areas:

```text
Install        25
Detection      35
Uninstall      25
Suitability    15
              ---
              100
```

Example:

```text
Packaging confidence               91 / 100 · High

✓ Install       Silent installer available
✓ Detection     Apps & Features + version
! Uninstall     ARP-based uninstall
✓ Suitability   x64 · machine · offline
```

Low-confidence applications should remain usable but require manual review.

Unsupported packages should not result in guessed or unreliable automation.

### Generated application

Inpakker may generate:

```text
source/
├── setup.exe
├── Install.ps1
├── Detect.ps1
└── Uninstall.ps1
```

along with the application configuration required to remember:

* WinGet package ID
* packaged version
* installer architecture
* installer scope
* installer type
* installer hash

This provenance enables future update detection.

### Sandbox + WinGet

This is why Sandbox comes first.

The eventual flow becomes:

```text
Search WinGet
      ↓
Packaging confidence
      ↓
Generate application
      ↓
Build
      ↓
Sandbox test
      ↓
✓ Install
✓ Detect
✓ Uninstall
      ↓
Ready for Intune
```

Static metadata confidence and real runtime testing remain intentionally separate.

---

# 🔄 4. Application update awareness

**Status: Planned**

Applications created from known sources such as WinGet should be able to tell Inpakker where they came from.

That makes update checking possible.

A future command might look like:

```powershell
inpakker updates
```

with output such as:

```text
APPLICATION          CURRENT     AVAILABLE
Mozilla Firefox      143.0       144.0
7-Zip                26.00       26.01
VLC                  3.0.21      3.0.22
```

From the TUI, applications with updates could be clearly marked.

Possible future workflow:

```text
New version available
      ↓
Review upstream changes
      ↓
Download new installer
      ↓
Update package metadata
      ↓
Build
      ↓
Sandbox test
      ↓
Ready
```

The goal is to make updates reproducible rather than silently replacing package contents.

Future source providers could potentially include:

* WinGet
* Direct URLs
* GitHub Releases
* Evergreen

These are ideas, not currently committed implementations.

---

# ☁️ 5. Microsoft Intune publishing

**Status: Longer term**

Eventually Inpakker may continue the workflow all the way into Intune.

Conceptually:

```text
Source
  ↓
Package
  ↓
Validate
  ↓
Sandbox test
  ↓
Publish
  ↓
Microsoft Intune
```

Potential functionality includes:

* Microsoft Graph authentication
* Workspace-to-tenant association
* Upload new Win32 applications
* Update existing application content
* Detection rules
* Requirements
* Return codes
* Dependencies
* Supersedence
* Assignments
* Dry-run / change preview
* Application metadata comparison

For environments managing multiple customers, each workspace could eventually be associated with a different Intune tenant.

Credentials and tokens must remain protected local data and will never belong in portable workspace configuration or Git repositories.

The goal is not to hide what Inpakker changes.

Publishing should be reviewable and predictable.

---

# 🔭 Exploring

These ideas may make sense later, but are intentionally behind the main roadmap:

### PSAppDeployToolkit support

Create or recognize PSADT-based application templates and make complex application deployment easier.

### Additional package sources

Provider architecture could eventually support:

```text
Local installer
WinGet
Direct URL
GitHub Release
Evergreen
```

### CI / headless workflows

Make it easy to validate and build a workspace from GitHub Actions, Azure DevOps or another CI environment.

### Package comparison

Compare the application source/configuration, built package and potentially the version currently published to Intune.

### Import existing Intune applications

Potentially turn an existing Intune Win32 application into an Inpakker-managed application definition where enough metadata can be recovered.

These are exploratory ideas, not promised features.

---

# 🧭 Direction

The intended evolution of Inpakker is:

```text
                    INPAKKER

                       │
                       ▼
                Organize apps
                       │
                       ▼
                    Validate
                       │
                       ▼
                     Build
                       │
                       ▼
              Test in Sandbox
                       │
                       ▼
             Source from WinGet
                       │
                       ▼
                Track updates
                       │
                       ▼
               Publish to Intune
```

The focus throughout remains:

**local-first · Git-friendly · transparent · reproducible · keyboard-friendly**

Inpakker should help automate repetitive packaging work without hiding enough of the process that an administrator no longer knows what will happen.
