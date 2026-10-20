<#
.SYNOPSIS
Copies a root-only Home GW backup archive over SSH and saves only DPAPI-encrypted
bytes on this Windows user profile. No plaintext archive is written to disk.

.EXAMPLE
./save-offhost-windows.ps1 -SshConfig C:\path\to\ssh.conf `
  -RemoteArchive /opt/oxnetwork-backups/oxnetwork-offhost-20261020T1843Z.tar `
  -ExpectedSha256 B5B153C71EB954845F0AD83C25E39606F166E4046678229AA32827948D491A07 `
  -Destination C:\path\to\outputs\oxnetwork-offhost-20261020T1843Z.tar.dpapi

The encrypted file requires this Windows user's DPAPI keys for recovery. Keep a
separately recoverable copy for a true machine-loss backup.
#>
param(
    [Parameter(Mandatory = $true)][string]$SshConfig,
    [Parameter(Mandatory = $true)][string]$RemoteArchive,
    [Parameter(Mandatory = $true)][string]$ExpectedSha256,
    [Parameter(Mandatory = $true)][string]$Destination,
    [string]$HostAlias = 'oxnet-transfer-gw'
)

$ErrorActionPreference = 'Stop'
if ($RemoteArchive -cnotmatch '^/opt/oxnetwork-backups/[A-Za-z0-9._/-]+\.tar$') {
    throw 'RemoteArchive must be a tar file under /opt/oxnetwork-backups.'
}
if ($RemoteArchive.Split('/') -contains '..') {
    throw 'RemoteArchive cannot contain a parent-directory segment.'
}
if ($ExpectedSha256 -notmatch '^[0-9a-fA-F]{64}$') {
    throw 'ExpectedSha256 must be a 64-character SHA-256 digest.'
}
if (-not [IO.Path]::IsPathFullyQualified($Destination) -or -not $Destination.EndsWith('.dpapi')) {
    throw 'Destination must be an absolute .dpapi path.'
}
if (Test-Path -LiteralPath $Destination) {
    throw 'The destination already exists; choose a new name instead of overwriting a backup.'
}
$parent = Split-Path -Parent $Destination
[void][IO.Directory]::CreateDirectory($parent)
$partial = "$Destination.partial"
if (Test-Path -LiteralPath $partial) {
    throw 'A partial backup already exists at the destination.'
}

$start = [System.Diagnostics.ProcessStartInfo]::new()
$start.FileName = 'ssh'
$start.UseShellExecute = $false
$start.RedirectStandardOutput = $true
$start.RedirectStandardError = $true
@('-F', $SshConfig, '-o', 'BatchMode=yes', $HostAlias, "cat $RemoteArchive") | ForEach-Object {
    [void]$start.ArgumentList.Add($_)
}

$process = [System.Diagnostics.Process]::Start($start)
$memory = [System.IO.MemoryStream]::new()
$plain = $null
$checked = $null
try {
    $process.StandardOutput.BaseStream.CopyTo($memory)
    $process.WaitForExit()
    if ($process.ExitCode -ne 0) {
        throw "SSH backup transfer failed: $($process.StandardError.ReadToEnd())"
    }
    $plain = $memory.ToArray()
    $actual = [Convert]::ToHexString([System.Security.Cryptography.SHA256]::HashData($plain))
    if ($actual -ne $ExpectedSha256.ToUpperInvariant()) {
        throw 'Transfer SHA-256 differs from the Home GW archive.'
    }
    $entropy = [System.Text.Encoding]::UTF8.GetBytes('oxnetwork-offhost-backup-v1')
    $encrypted = [System.Security.Cryptography.ProtectedData]::Protect(
        $plain, $entropy, [System.Security.Cryptography.DataProtectionScope]::CurrentUser)
    [System.IO.File]::WriteAllBytes($partial, $encrypted)
    $checked = [System.Security.Cryptography.ProtectedData]::Unprotect(
        [System.IO.File]::ReadAllBytes($partial), $entropy,
        [System.Security.Cryptography.DataProtectionScope]::CurrentUser)
    $checkedHash = [Convert]::ToHexString([System.Security.Cryptography.SHA256]::HashData($checked))
    if ($checkedHash -ne $actual) {
        throw 'The encrypted copy failed its decrypt-and-hash check.'
    }
    [System.IO.File]::Move($partial, $Destination)
    Write-Output "Encrypted backup verified: $Destination"
    Write-Output "Plaintext SHA-256: $actual; bytes: $($plain.Length)"
} finally {
    if ($null -ne $plain) { [Array]::Clear($plain, 0, $plain.Length) }
    if ($null -ne $checked) { [Array]::Clear($checked, 0, $checked.Length) }
    $memory.Dispose()
    $process.Dispose()
    if (Test-Path -LiteralPath $partial) { Remove-Item -LiteralPath $partial -Force }
}
