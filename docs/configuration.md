# Configuration

Inpakker commands run from a workspace containing `inpakker.config.json`.
Interactive `inpakker setup` is the easiest way to create one.

## Unattended setup

All setup answers are also available as flags:

```powershell
inpakker setup C:\Packages `
  --apps-dir apps `
  --output-dir output `
  --intune-util C:\Tools\IntuneWinAppUtil.exe `
  --decoder C:\Tools\IntuneWinAppUtilDecoder.exe `
  --no-input
```

Add `--no-example` if the workspace should start empty.

## Workspace configuration

`inpakker.config.json` supports:

```json
{
  "intunewinapputil": "C:\\Tools\\IntuneWinAppUtil.exe",
  "decoderPath": "C:\\Tools\\IntuneWinAppUtilDecoder.exe",
  "defaultOutputDir": "output",
  "appsDir": "apps",
  "muteIntuneWinAppUtil": false
}
```

- `intunewinapputil`: full path to Microsoft's `IntuneWinAppUtil.exe`. The
  legacy key `intuneWinAppUtilPath` is still accepted.
- `decoderPath`: optional full path to the separately installed
  `IntuneWinAppUtilDecoder.exe` used by `unpack`. Its upstream project provides
  compiled files in the checked-in
  [`bin/Release` folder](https://github.com/okieselbach/Intune/tree/master/IntuneWinAppUtilDecoder/IntuneWinAppUtilDecoder/bin/Release).
- `defaultOutputDir`: default output directory inside each application.
- `appsDir`: relative workspace directory containing applications and groups.
- `muteIntuneWinAppUtil`: suppresses output from Microsoft's packaging tool
  when `true`.

## Application configuration

Each application directory contains `app.config.json`:

```json
{
  "name": "myapp",
  "displayName": "My Application",
  "source": "source",
  "setupFile": "setup.exe",
  "installCommand": "",
  "uninstallCommand": "",
  "outputDir": "output"
}
```

- `name`: internal application identifier, normally matching its directory.
- `displayName`: friendly name shown in Inpakker.
- `source`: relative directory containing every file that should be packaged.
- `setupFile`: setup executable or script within the source directory.
- `installCommand` and `uninstallCommand`: reserved for future functionality;
  they are not passed to the packaging utility today.
- `outputDir`: optional application-specific output directory. When omitted,
  `defaultOutputDir` is used.

Application, group, source, setup, and output paths must remain inside their
documented roots. Run `inpakker validate` after editing configuration by hand.
