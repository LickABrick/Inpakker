# Inpakker

Inpakker is a Windows CLI and terminal interface for organizing, validating,
packaging, and inspecting Win32 applications for Microsoft Intune. Packaging is
delegated to Microsoft's `IntuneWinAppUtil.exe`; optional unpacking is delegated
to the separately installed `IntuneWinAppUtilDecoder.exe` project.

## Quick Start

1. Download the latest Inpakker executable from the [releases page](https://github.com/LickABrick/Inpakker/releases).

2. Prepare your workspace folder structure:

    ```shell
    workspace/
      ├─ inpakker.config.json
      └─ apps/
         ├─ app1/
         │  ├─ app.config.json
         │  └─ source/
         │     └─ setup.exe
         └─ app2/
            └─ app.config.json
    ```

3. Edit the global and app configuration files as needed (see below).

4. Open a terminal, navigate to your workspace folder, then run:

    ```bash
    path/to/inpakker build app1 app2
    ```

   Run `inpakker` without arguments in an interactive terminal to open the TUI.
   Scripts and redirected sessions receive command help instead.

*Note:* Replace `path/to/inpakker` with the path to the downloaded executable. On Windows, this might be `.\inpakker.exe`.

## Configuration Reference

### Global Configuration (`inpakker.config.json`)

```json
{
  "intunewinapputil": "C:\\Tools\\IntuneWinAppUtil.exe",
  "decoderPath": "C:\\Tools\\IntuneWinAppUtilDecoder.exe",
  "defaultOutputDir": "output",
  "appsDir": "apps",
  "muteIntuneWinAppUtil": false
}
```

* **`intunewinapputil`**
  Full path to the `IntuneWinAppUtil.exe` executable used for packaging.
  The legacy key `intuneWinAppUtilPath` remains accepted for compatibility.

* **`decoderPath`**
  Optional full path to `IntuneWinAppUtilDecoder.exe`. This is required only by
  `unpack` and the TUI unpack action. Inpakker does not redistribute the decoder.

* **`defaultOutputDir`**
  Default output folder inside each app folder for the generated `.intunewin` file.

* **`appsDir`**
  Root directory containing all app folders.

* **`muteIntuneWinAppUtil`**
  Suppresses the wrapped packaging tool's stdout and stderr when `true`.

---

### App Configuration (`app.config.json`)

```json
{
  "name": "myapp",
  "displayName": "My Awesome App",
  "source": "source",
  "setupFile": "setup.exe",
  "installCommand": "",
  "uninstallCommand": "",
  "outputDir": "output"
}
```

* **`name`**
  Internal identifier for the app, usually matching the app folder name.

* **`displayName`**
  Friendly name for UI or logs.

* **`source`**
  Relative path inside the app folder pointing to the source files.

* **`setupFile`**
  Setup executable filename inside the source folder.

* **`installCommand`** and **`uninstallCommand`**
  Reserved for future use.

* **`outputDir`**
  Overrides global output folder if specified.

## Notes

* Paths are relative to the location of the config files.
* The global config (`inpakker.config.json`) **must** be in your workspace root.
* Each app folder inside `appsDir` requires its own `app.config.json`.

## Commands

Create an application scaffold under the configured `appsDir`, optionally in a
group:

```shell
inpakker new myapp
inpakker new myapp --group browsers --display-name "My App" --setup-file setup.exe
```

The command refuses to overwrite an existing `app.config.json`. Add the setup
file and set `setupFile` before validating or building the app.

Validate every app recursively under `appsDir`:

```shell
inpakker validate
inpakker validate browsers/myapp
```

Validation checks required fields, relative paths, source directories, and
setup files. Invalid apps are listed and cause a non-zero exit status.

Build named apps or immediate app groups, or build the complete workspace:

```shell
inpakker build app1 app2
inpakker build --all
```

Build output reports individual failures and one final summary instead of one
status line per successful or skipped app. Any packaging failure causes a
non-zero exit status.

List applications or show one application's details:

```shell
inpakker list
inpakker list browsers --json
inpakker show browsers/myapp
```

`list --json` and `show --json` provide machine-readable output.

### Unpacking

The decoder is maintained separately by Oliver Kieselbach. Download
`IntuneWinAppUtilDecoder.zip` or the executable from the project's
[checked-in `bin/Release` folder](https://github.com/okieselbach/Intune/tree/master/IntuneWinAppUtilDecoder/IntuneWinAppUtilDecoder/bin/Release),
review that third-party project as appropriate for your environment, extract it,
and set `decoderPath` to the full path of `IntuneWinAppUtilDecoder.exe`. The
current decoder project targets .NET Framework 4.6.1 and is intended to run on
Windows.

Unpack the single package produced for an app, a direct package path, or all
apps that currently have exactly one package:

```shell
inpakker unpack browsers/myapp
inpakker unpack C:\\Packages\\myapp.intunewin --destination C:\\Decoded\\myapp
inpakker unpack --all
```

By default output is written to `decoded/<package-name>` beside the package.
Existing destinations are preserved unless `--force` is supplied. Inpakker
stages decoder execution in a temporary directory and rejects unsafe paths in
the resulting ZIP archive.

### Terminal interface

Run `inpakker` with no arguments, or run `inpakker tui`, to open the terminal
interface. It provides fuzzy search, application status and details, guided app
creation, refresh, and validate/build/unpack actions for either the selected app
or the complete workspace. Its key hints are shown at the bottom of each view.

Every TUI operation has a CLI equivalent: `list`, `show`, `new`, `validate`,
`build`, and `unpack`. Destructive workspace operations and raw configuration
editing are intentionally not included; use a version-controlled editor for
those tasks.

## Version

Display the version embedded in a release build with:

```shell
inpakker --version
```

Local development builds report `dev`. Tagged releases use Semantic Versioning
and are published on GitHub with a Windows AMD64 ZIP and a SHA-256 checksum
manifest.

Building v0.2 and later from source requires Go 1.25 or newer.

## Releasing

After merging a completed version branch into `master`, create and push an
annotated Semantic Version tag from the release commit:

```shell
git switch master
git pull --ff-only
git tag -a v1.2.3 -m "Release v1.2.3"
git push origin v1.2.3
```

The release workflow rejects lightweight tags, invalid version tags, and tags
whose commits are not contained in `master`. A valid tag runs the verification
suite and publishes the GitHub Release automatically.
