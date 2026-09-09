# Inpakker

Inpakker is a simple, standalone CLI tool that wraps Microsoft's `IntuneWinAppUtil.exe` to help you organize and package multiple Win32 apps for Microsoft Intune deployment.

## Quick Start

1. Download the latest Inpakker executable from the [releases page](https://github.com/LickABrick/Inpakker/releases).

2. Prepare your workspace folder structure:

    ```shell
    workspace/
      ├─ inpakker.config.json
      └─ apps/
      ├─ app1/
      │   └─ app.config.json
      └─ app2/
      └─ app.config.json
    ```

3. Edit the global and app configuration files as needed (see below).

4. Open a terminal, navigate to your workspace folder, then run:

    ```bash
    path/to/inpakker build app1 app2
    ````

*Note:* Replace `path/to/inpakker` with the path to the downloaded executable. On Windows, this might be `.\inpakker.exe`.

## Configuration Reference

### Global Configuration (`inpakker.config.json`)

```json
{
  "intunewinapputil": "C:\\Tools\\IntuneWinAppUtil.exe",
  "defaultOutputDir": "output",
  "appsDir": "apps"
}
```

* **`intunewinapputil`**
  Full path to the `IntuneWinAppUtil.exe` executable used for packaging.

* **`defaultOutputDir`**
  Default output folder inside each app folder for the generated `.intunewin` file.

* **`appsDir`**
  Root directory containing all app folders.

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

## Version

Display the version embedded in a release build with:

```shell
inpakker --version
```

Local development builds report `dev`. Tagged releases use Semantic Versioning
and are published on GitHub with a Windows AMD64 ZIP and a SHA-256 checksum
manifest.

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
