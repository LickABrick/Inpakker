<p align="center">
  <img src="docs/assets/inpakker-logo.png" width="180" alt="Inpakker parcel box logo">
</p>

<h1 align="center">📦 Inpakker</h1>

<p align="center">
  <strong>Build, organize and test Microsoft Intune Win32 applications without making packaging your full-time job.</strong>
</p>

Inpakker is a Windows terminal application for managing **Microsoft Intune Win32 app packages**.

Keep your applications organized in workspaces, validate them before packaging, build `.intunewin` files with Microsoft's Content Prep Tool, and quickly see what needs attention — all from one keyboard-friendly interface.

Whether you maintain a handful of applications or separate app libraries for multiple customers, Inpakker aims to make the packaging workflow **repeatable, tidy and fast**.

> Inpakker currently supports Windows AMD64.

---

## ✨ What can Inpakker do?

* 📦 **Organize apps in workspaces**
  Keep separate packaging environments for customers, teams or projects.

* 🗂️ **Group applications**
  Use folders such as `Browsers`, `Microsoft/Office` or whatever structure makes sense for you.

* ✅ **Validate before you build**
  Catch missing installers, invalid paths and configuration problems early.

* ⚡ **Smart builds**
  Applications that have not changed do not need to be packaged again.

* 🔨 **Build `.intunewin` packages**
  Inpakker uses Microsoft's official Win32 Content Prep Tool.

* 🔍 **Inspect package state**
  See at a glance which apps are valid, need a build or are already up to date.

* 📂 **Jump straight into folders**
  Open workspaces, applications and package output directly from the TUI.

* 📤 **Unpack `.intunewin` files**
  Optional decoder support makes inspecting existing packages easy.

* ⌨️ **Use the TUI or CLI**
  Work interactively, or use commands in scripts and automation.

---

## 🚀 Install

Open PowerShell:

```powershell
irm https://raw.githubusercontent.com/LickABrick/Inpakker/master/install.ps1 | iex
```

Then start Inpakker:

```powershell
inpakker
```

No administrator privileges are required to install Inpakker itself.

Already installed? Running the installer again updates you to the latest stable release.

---

## 🎬 Getting started

Start the TUI:

```powershell
inpakker
```

On first launch you can:

1. **Create a workspace**
2. **Add an existing workspace**
3. Start adding applications

A workspace could represent:

```text
📦 Shared Apps
📦 Customer A
📦 Customer B
📦 Lab
```

Inside a workspace, applications can be organized however you like:

```text
apps/
├── Browsers/
│   ├── Mozilla-Firefox/
│   └── Google-Chrome/
├── Microsoft/
│   ├── Teams/
│   └── Office/
└── Utilities/
    ├── 7-Zip/
    └── Notepad++/
```

Inpakker takes care of the structure around each application.

---

## 📦 A typical application

An application might look like this:

```text
Mozilla-Firefox/
├── inpakker.app.json
├── source/
│   ├── Firefox Setup.exe
│   ├── Install.ps1
│   └── Uninstall.ps1
└── output/
    └── Firefox Setup.intunewin
```

The exact `source` and `output` directory names can be customized per workspace.

Your workspace and application configuration stay portable, while machine-specific Inpakker settings stay outside the workspace.

---

## 🛠️ Building an application

From the TUI, select an application and build it.

Or from PowerShell:

```powershell
inpakker build "Mozilla Firefox"
```

Build everything:

```powershell
inpakker build --all
```

Need a clean rebuild?

```powershell
inpakker build "Mozilla Firefox" --force
```

Inpakker keeps track of what went into a package, so unchanged applications can be skipped automatically.

---

## ✅ Validate first

Before packaging:

```powershell
inpakker validate "Mozilla Firefox"
```

Or validate everything:

```powershell
inpakker validate --all
```

Validation checks the application configuration and packaging source before the Content Prep Tool is started.

That means fewer surprises halfway through a build.

---

## ⌨️ Terminal interface

The TUI is the easiest way to use Inpakker.

Some useful shortcuts:

| Key                 | Action                  |
| ------------------- | ----------------------- |
| `Enter`             | Open / select           |
| `/`                 | Filter current list     |
| `:`                 | Command palette         |
| `n`                 | New application         |
| `a`                 | Actions                 |
| `b`                 | Build                   |
| `B`                 | Build options           |
| `v`                 | Validate                |
| `u`                 | Unpack                  |
| `o`                 | Open folder             |
| `r`                 | Refresh                 |
| `w`                 | Workspaces              |
| `s`                 | Settings                |
| `i`                 | About                   |
| `?`                 | Contextual help         |
| `Esc` / `Backspace` | Back                    |
| `Ctrl+C`            | Cancel active operation |

The TUI is designed to remain fully usable without memorizing every shortcut — use `?` whenever you need a reminder.

👉 See the full [TUI guide](docs/tui.md).

---

## 🧰 External tools

Inpakker uses external tools for some operations:

**Microsoft Win32 Content Prep Tool**
Required to build `.intunewin` packages.

**IntuneWinAppUtilDecoder**
Optional. Used to unpack existing `.intunewin` packages.

You can set these up directly from **Settings** in the TUI.

Inpakker can download supported tools from their upstream project after you accept the relevant license, or you can point Inpakker at an executable you already have.

---

## 💻 Prefer the CLI?

The TUI and CLI work with the same workspaces and applications.

A few examples:

```powershell
# List applications
inpakker list

# Create an application
inpakker new "Mozilla Firefox" --group Browsers --setup-file "Firefox Setup.exe"

# Build an application
inpakker build "Mozilla Firefox"

# Build everything
inpakker build --all

# Validate everything
inpakker validate --all

# Open an application folder
inpakker open "Mozilla Firefox"

# Diagnose the current workspace
inpakker doctor
```

Every command has built-in help:

```powershell
inpakker build --help
```

---

## 🗂️ Workspaces

Workspaces keep packaging environments independent from each other.

That makes it easy to use Inpakker for things such as:

```text
Shared applications
Customer-specific applications
Test/lab packages
Different teams or environments
```

Switch workspaces from the TUI with `w`, or from the CLI:

```powershell
inpakker workspace list
inpakker workspace use "Customer A"
```

Removing a workspace from Inpakker does **not** delete the workspace or its files.

For configuration details, inheritance and workspace settings, see [Configuration](docs/configuration.md).

---

## 🔓 Unpacking packages

With the optional decoder configured:

```powershell
inpakker unpack "Mozilla Firefox"
```

You can also unpack an existing package directly:

```powershell
inpakker unpack C:\Packages\firefox.intunewin --destination C:\Decoded\firefox
```

This is useful when you need to inspect what is actually inside an Intune package.

---

## 🗺️ What's next?

Inpakker is still growing.

Some of the bigger ideas on the roadmap include:

**🧪 Windows Sandbox testing**
Launch built applications in a clean Windows Sandbox and test them as SYSTEM before deploying them.

**📦 WinGet integration**
Search the WinGet catalog and let Inpakker turn suitable packages into reproducible Intune applications.

**🔄 Application update awareness**
See when packaged applications have newer versions available.

**☁️ Intune publishing**
Eventually publish and update applications directly in Microsoft Intune.

See the full [Roadmap](ROADMAP.md).

---

## 📚 Documentation

Need more detail?

* [Terminal interface](docs/tui.md)
* [Configuration](docs/configuration.md)
* [Troubleshooting](docs/troubleshooting.md)
* [Roadmap](ROADMAP.md)

For command-specific help:

```powershell
inpakker <command> --help
```

---

## 🔄 Updating

Check for updates from **About** in the TUI, or:

```powershell
inpakker update --check
```

Install an available update:

```powershell
inpakker update
```

---

## 🗑️ Uninstall

Run:

```powershell
irm https://raw.githubusercontent.com/LickABrick/Inpakker/master/uninstall.ps1 | iex
```

Your workspaces are left alone.

---

## 🤝 Contributing

Ideas, bug reports and contributions are welcome.

See [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.

Found a security issue? Please follow [SECURITY.md](SECURITY.md).

---

## 📜 License

Inpakker is available under the [MIT License](LICENSE).

Inpakker is an independent project and is not affiliated with Microsoft. External tools keep their own licenses and terms.
