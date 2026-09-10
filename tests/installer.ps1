# Isolated tests; never touch real user PATH, install directories or workspace data.
$ErrorActionPreference = 'Stop'
$temporary = Join-Path ([IO.Path]::GetTempPath()) ('inpakker-script-test-' + [Guid]::NewGuid())
New-Item -ItemType Directory -Path $temporary | Out-Null
try {
    . (Join-Path $PSScriptRoot '..\install.ps1') -InstallDir $temporary
    . (Join-Path $PSScriptRoot '..\uninstall.ps1') -InstallDir $temporary
    $install = Join-Path $temporary 'Programs\Inpakker'
    $path = Add-InpakkerPath "C:\Tools;$install\;$($install.ToUpperInvariant())" $install
    if (@($path -split ';' | Where-Object { (Normalize-InpakkerPath $_) -eq (Normalize-InpakkerPath $install) }).Count -ne 1) { throw 'PATH duplicate prevention failed.' }
    if ((Remove-InpakkerPath $path $install) -ne 'C:\Tools') { throw 'PATH removal failed.' }
    $release = @{tag_name='v0.4.0';draft=$false;prerelease=$false;assets=@()}
    foreach ($name in @('inpakker_v0.4.0_windows_amd64.zip','inpakker_v0.4.0_checksums.txt','inpakker_v0.4.0_checksums.txt.sig')) {
        $release.assets += @{name=$name;browser_download_url="https://github.com/LickABrick/Inpakker/releases/download/v0.4.0/$name"}
    }
    $assets = Get-InpakkerAssets $release
    if ($assets.Archive -ne 'inpakker_v0.4.0_windows_amd64.zip') { throw 'Asset selection failed.' }
    $release.prerelease = $true
    $rejected = $false
    try { Get-InpakkerAssets $release | Out-Null } catch { $rejected = $true }
    if (-not $rejected) { throw 'Accepted prerelease.' }
    $rejected = $false
    try { Confirm-InpakkerSignature ([byte[]](1,2,3)) ([byte[]](1,2,3)) } catch { $rejected = $true }
    if (-not $rejected) { throw 'Accepted invalid signature.' }
    $workspace = Join-Path $temporary 'Customer Workspace'
    $state = Join-Path $temporary 'Inpakker'
    New-Item -ItemType Directory -Path $install,$workspace,$state | Out-Null
    Set-Content -LiteralPath (Join-Path $install 'inpakker.exe') -Value 'fixture'
    Set-Content -LiteralPath (Join-Path $workspace 'source.txt') -Value 'preserve'
    @{schemaVersion=1;workspaces=@(@{path=$workspace})} | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $state 'config.json')
    Remove-InpakkerInstallation $install
    if (-not (Test-Path -LiteralPath (Join-Path $state 'config.json'))) { throw 'Default uninstall removed config.' }
    if (-not (Test-Path -LiteralPath (Join-Path $workspace 'source.txt'))) { throw 'Default uninstall removed workspace.' }
    Remove-InpakkerState $state
    if (Test-Path -LiteralPath $state) { throw 'Purge did not remove state.' }
    if (-not (Test-Path -LiteralPath (Join-Path $workspace 'source.txt'))) { throw 'Purge removed workspace.' }
    Write-Host 'Installer/uninstaller tests passed.'
} finally { Remove-Item -LiteralPath $temporary -Recurse -Force }
