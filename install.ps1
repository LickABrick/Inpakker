#Requires -Version 5.1
[CmdletBinding()]
param(
    [string]$Version,
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'Programs\Inpakker'),
    [switch]$Force
)

function Normalize-InpakkerPath([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($Path)) { return '' }
    return [IO.Path]::GetFullPath([Environment]::ExpandEnvironmentVariables($Path.Trim().Trim('"'))).TrimEnd('\', '/').ToUpperInvariant()
}

function Add-InpakkerPath([string]$Value, [string]$Directory) {
    $target = Normalize-InpakkerPath $Directory
    $entries = @($Value -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    $kept = @($entries | Where-Object { (Normalize-InpakkerPath $_) -ne $target })
    return (($kept + $Directory) -join ';')
}

function Get-InpakkerAssets($Release) {
    if ($Release.draft -or $Release.prerelease -or $Release.tag_name -notmatch '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$') {
        throw 'Expected a stable vMAJOR.MINOR.PATCH release.'
    }
    $archive = "inpakker_$($Release.tag_name)_windows_amd64.zip"
    $checksums = "inpakker_$($Release.tag_name)_checksums.txt"
    $result = @{}
    foreach ($name in @($archive, $checksums, "$checksums.sig")) {
        $matches = @($Release.assets | Where-Object { $_.name -ceq $name })
        if ($matches.Count -ne 1) { throw "Release requires exactly one asset named $name." }
        $uri = [Uri]$matches[0].browser_download_url
        if ($uri.Scheme -ne 'https' -or $uri.Host -ne 'github.com' -or -not $uri.AbsolutePath.StartsWith('/LickABrick/Inpakker/releases/download/', [StringComparison]::OrdinalIgnoreCase)) {
            throw 'Release asset URL is not an official Inpakker GitHub download.'
        }
        $result[$name] = $uri.AbsoluteUri
    }
    return @{ Archive = $archive; Manifest = $checksums; URLs = $result }
}

function Confirm-InpakkerSignature([byte[]]$Manifest, [byte[]]$Signature) {
    # Same pinned public certificate as internal/updater/release-signing-cert.pem.
    $certificateBase64 = 'MIIBnTCCAUOgAwIBAgIUdI9VM54oOvf4Fzh7QI8cxNUj7T8wCgYIKoZIzj0EAwIwIzEhMB8GA1UEAwwYSW5wYWtrZXItUmVsZWFzZS1TaWduaW5nMCAXDTI2MDkwOTEzMTUwOVoYDzIxMjYwODE2MTMxNTA5WjAjMSEwHwYDVQQDDBhJbnBha2tlci1SZWxlYXNlLVNpZ25pbmcwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAQ2/dXwMJIOeJPsQQV2BsPwbU+ddCWRE/UjXE4n99Zwxs5G9LqSk2eE4dBkfaab5oWT0FN4t4shkIHh9z5AoFP6o1MwUTAdBgNVHQ4EFgQU0dBabMLGYGfCZTbJmjJdJ7ZNNeQwHwYDVR0jBBgwFoAU0dBabMLGYGfCZTbJmjJdJ7ZNNeQwDwYDVR0TAQH/BAUwAwEB/zAKBggqhkjOPQQDAgNIADBFAiEApA58mEk66qNFRcleziDCUwveZliU/SLGr7BXd5oMJBcCICm/hHk3rPPSgFvAa0XANSdSU0t4Ta2BDs4RmyktVFVY'
    $certificate = [Security.Cryptography.X509Certificates.X509Certificate2]::new([Convert]::FromBase64String($certificateBase64))
    $key = [Security.Cryptography.X509Certificates.ECDsaCertificateExtensions]::GetECDsaPublicKey($certificate)
    try {
        # OpenSSL emits DER; .NET Framework ECDsaCng VerifyData expects fixed-width P1363.
        if ($Signature.Length -lt 8 -or $Signature.Length -gt 72 -or $Signature[0] -ne 0x30 -or $Signature[1] -ne ($Signature.Length - 2)) { throw 'Malformed checksum signature.' }
        $fixed = New-Object byte[] 64
        $offset = 2
        for ($component = 0; $component -lt 2; $component++) {
            if ($offset + 2 -gt $Signature.Length -or $Signature[$offset] -ne 2) { throw 'Malformed ECDSA integer.' }
            $length = [int]$Signature[$offset + 1]
            $offset += 2
            if ($length -lt 1 -or $length -gt 33 -or $offset + $length -gt $Signature.Length) { throw 'Malformed ECDSA length.' }
            if ($Signature[$offset] -ge 128) { throw 'Negative ECDSA integer.' }
            if ($length -eq 33) {
                if ($Signature[$offset] -ne 0) { throw 'Oversized ECDSA integer.' }
                $offset++; $length--
            }
            [Array]::Copy($Signature, $offset, $fixed, ($component * 32 + 32 - $length), $length)
            $offset += $length
        }
        if ($offset -ne $Signature.Length -or -not $key.VerifyData($Manifest, $fixed, [Security.Cryptography.HashAlgorithmName]::SHA256)) { throw 'Checksum manifest signature is not trusted.' }
    } finally { if ($key) { $key.Dispose() }; $certificate.Dispose() }
}

function Confirm-InpakkerChecksum([string]$Archive, [string]$Name, [byte[]]$Manifest) {
    $lines = [Text.Encoding]::UTF8.GetString($Manifest) -split "`n"
    $hashes = @()
    foreach ($line in $lines) {
        if ($line.Trim() -match '^([0-9a-fA-F]{64})\s+\*?(.+)$' -and $Matches[2] -ceq $Name) { $hashes += $Matches[1] }
    }
    if ($hashes.Count -ne 1) { throw 'Manifest must contain exactly one checksum for the archive.' }
    if ((Get-FileHash -LiteralPath $Archive -Algorithm SHA256).Hash -ne $hashes[0]) { throw 'Archive SHA-256 does not match the signed manifest.' }
}

function Install-Inpakker([string]$RequestedVersion, [string]$Directory, [bool]$Replace) {
    if ($env:OS -ne 'Windows_NT' -or -not [Environment]::Is64BitOperatingSystem -or ($env:PROCESSOR_ARCHITECTURE -ne 'AMD64' -and $env:PROCESSOR_ARCHITEW6432 -ne 'AMD64')) { throw 'Inpakker supports Windows AMD64.' }
    $Directory = [IO.Path]::GetFullPath($Directory)
    if ((Normalize-InpakkerPath $Directory) -eq (Normalize-InpakkerPath ([IO.Path]::GetPathRoot($Directory)))) { throw 'Install directory cannot be a filesystem root.' }
    if ($RequestedVersion -and $RequestedVersion -notmatch '^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$') { throw 'Version must be a stable MAJOR.MINOR.PATCH.' }
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    $endpoint = 'https://api.github.com/repos/LickABrick/Inpakker/releases/latest'
    if ($RequestedVersion) { $endpoint = 'https://api.github.com/repos/LickABrick/Inpakker/releases/tags/v' + $RequestedVersion.TrimStart('v') }
    $release = Invoke-RestMethod -Uri $endpoint -Headers @{ 'User-Agent' = 'Inpakker-Installer' } -TimeoutSec 30
    $assets = Get-InpakkerAssets $release
    $temporary = Join-Path ([IO.Path]::GetTempPath()) ('inpakker-install-' + [Guid]::NewGuid())
    $staged = $null
    New-Item -ItemType Directory -Path $temporary -ErrorAction Stop | Out-Null
    try {
        foreach ($name in $assets.URLs.Keys) {
            Invoke-WebRequest -UseBasicParsing -Uri $assets.URLs[$name] -OutFile (Join-Path $temporary $name) -TimeoutSec 120
            if ((Get-Item -LiteralPath (Join-Path $temporary $name)).Length -eq 0) { throw "Empty download: $name" }
        }
        $manifest = [IO.File]::ReadAllBytes((Join-Path $temporary $assets.Manifest))
        Confirm-InpakkerSignature $manifest ([IO.File]::ReadAllBytes((Join-Path $temporary ($assets.Manifest + '.sig'))))
        $archive = Join-Path $temporary $assets.Archive
        Confirm-InpakkerChecksum $archive $assets.Archive $manifest
        Add-Type -AssemblyName System.IO.Compression.FileSystem
        $zip = [IO.Compression.ZipFile]::OpenRead($archive)
        try {
            $executables = @($zip.Entries | Where-Object { $_.FullName -ceq 'inpakker.exe' })
            if ($executables.Count -ne 1 -or $executables[0].Length -lt 2 -or $executables[0].Length -gt 100MB) { throw 'Archive must contain one expected inpakker.exe.' }
            New-Item -ItemType Directory -Force -Path $Directory -ErrorAction Stop | Out-Null
            $staged = Join-Path $Directory ('.inpakker-' + [Guid]::NewGuid() + '.tmp')
            [IO.Compression.ZipFileExtensions]::ExtractToFile($executables[0], $staged, $false)
        } finally { $zip.Dispose() }
        $bytes = [IO.File]::ReadAllBytes($staged)
        if ($bytes[0] -ne 0x4D -or $bytes[1] -ne 0x5A) { throw 'Expected a Windows executable.' }
        $target = Join-Path $Directory 'inpakker.exe'
        if (Test-Path -LiteralPath $target) {
            $same = (Get-FileHash -LiteralPath $target).Hash -eq (Get-FileHash -LiteralPath $staged).Hash
            if (-not $same -or $Replace) {
                $backup = Join-Path $Directory ('.inpakker-backup-' + [Guid]::NewGuid())
                [IO.File]::Replace($staged, $target, $backup)
                Remove-Item -LiteralPath $backup -Force
            }
        } else { [IO.File]::Move($staged, $target) }
        $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        [Environment]::SetEnvironmentVariable('Path', (Add-InpakkerPath $userPath $Directory), 'User')
        $env:Path = Add-InpakkerPath $env:Path $Directory
        Write-Host "Installed Inpakker $($release.tag_name) at $target"
        Write-Host 'Run inpakker --version, then inpakker.'
    } finally {
        if ($staged -and (Test-Path -LiteralPath $staged)) { Remove-Item -LiteralPath $staged -Force }
        Remove-Item -LiteralPath $temporary -Recurse -Force
    }
}

if ($MyInvocation.InvocationName -ne '.') {
    $ErrorActionPreference = 'Stop'
    Install-Inpakker $Version $InstallDir $Force.IsPresent
}
