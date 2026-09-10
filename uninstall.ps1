#Requires -Version 5.1
[CmdletBinding()]
param(
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'Programs\Inpakker'),
    [switch]$Purge
)

function Normalize-InpakkerPath([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($Path)) { return '' }
    return [IO.Path]::GetFullPath([Environment]::ExpandEnvironmentVariables($Path.Trim().Trim('"'))).TrimEnd('\', '/').ToUpperInvariant()
}
function Remove-InpakkerPath([string]$Value, [string]$Directory) {
    $target = Normalize-InpakkerPath $Directory
    return (@($Value -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) -and (Normalize-InpakkerPath $_) -ne $target }) -join ';')
}
function Remove-InpakkerInstallation([string]$Directory) {
    # Delete only the owned executable. Unrelated files in custom install directories survive.
    $target = Join-Path $Directory 'inpakker.exe'
    if (Test-Path -LiteralPath $target -PathType Leaf) { Remove-Item -LiteralPath $target -Force -ErrorAction Stop }
    if ((Test-Path -LiteralPath $Directory -PathType Container) -and @(Get-ChildItem -LiteralPath $Directory -Force).Count -eq 0) { Remove-Item -LiteralPath $Directory -Force }
}
function Remove-InpakkerState([string]$StateDirectory) {
    if (-not (Test-Path -LiteralPath $StateDirectory)) { return }
    $root = Normalize-InpakkerPath $StateDirectory
    $config = Join-Path $StateDirectory 'config.json'
    if (Test-Path -LiteralPath $config) {
        $user = Get-Content -LiteralPath $config -Raw | ConvertFrom-Json -ErrorAction Stop
        foreach ($workspace in $user.workspaces) {
            $path = Normalize-InpakkerPath $workspace.path
            if ($path -eq $root -or $path.StartsWith($root + '\') -or $root.StartsWith($path + '\')) { throw 'Purge refused: workspace data overlaps the Inpakker state directory. Remove state manually while preserving that workspace.' }
        }
    }
    # Reparse points are not followed; refuse them anywhere in the state tree.
    if ((Get-Item -LiteralPath $StateDirectory -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Purge refused for a linked state directory.' }
    $pending = [Collections.Generic.Stack[string]]::new()
    $pending.Push($StateDirectory)
    while ($pending.Count -gt 0) {
        $currentDirectory = $pending.Pop()
        foreach ($item in Get-ChildItem -LiteralPath $currentDirectory -Force) {
            if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Purge refused: state contains a filesystem link.' }
            if ($item.Name -eq 'inpakker.workspace.json') { throw 'Purge refused: state contains a workspace.' }
            if ($item.PSIsContainer) { $pending.Push($item.FullName) }
        }
    }
    Remove-Item -LiteralPath $StateDirectory -Recurse -Force -ErrorAction Stop
}
function Uninstall-Inpakker([string]$Directory, [bool]$RemoveState) {
    if ($env:OS -ne 'Windows_NT') { throw 'This uninstaller supports Windows.' }
    Remove-InpakkerInstallation $Directory
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    [Environment]::SetEnvironmentVariable('Path', (Remove-InpakkerPath $userPath $Directory), 'User')
    $env:Path = Remove-InpakkerPath $env:Path $Directory
    if ($RemoveState) {
        $stateDirectory = Join-Path $env:LOCALAPPDATA 'Inpakker'
        if ($env:INPAKKER_HOME) { $stateDirectory = $env:INPAKKER_HOME }
        Remove-InpakkerState $stateDirectory
    }
    Write-Host 'Inpakker uninstalled. Workspace folders and their files are preserved.'
    if (-not $RemoveState) { Write-Host 'Global configuration and state are preserved.' }
}
if ($MyInvocation.InvocationName -ne '.') {
    $ErrorActionPreference = 'Stop'
    Uninstall-Inpakker $InstallDir $Purge.IsPresent
}
