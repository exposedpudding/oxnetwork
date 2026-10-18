param(
    [Parameter(Mandatory = $true)][ValidatePattern('^tailscale-[0-9]+$')][string]$NodeId,
    [Parameter(Mandatory = $true)][string]$Binary,
    [Parameter(Mandatory = $true)][string]$TokenFile
)

$ErrorActionPreference = 'Stop'
$binarySource = (Resolve-Path -LiteralPath $Binary).Path
$tokenSource = (Resolve-Path -LiteralPath $TokenFile).Path
$destination = Join-Path $env:LOCALAPPDATA 'oxnetwork\agent'
$binaryTarget = Join-Path $destination 'oxnet-host-agent.exe'
$tokenTarget = Join-Path $destination 'host-agent.token'

New-Item -ItemType Directory -Path $destination -Force | Out-Null
$security = New-Object System.Security.AccessControl.DirectorySecurity
$security.SetAccessRuleProtection($true, $false)
$user = [System.Security.Principal.WindowsIdentity]::GetCurrent().User
$system = New-Object System.Security.Principal.SecurityIdentifier('S-1-5-18')
foreach ($identity in @($user, $system)) {
    $rule = New-Object System.Security.AccessControl.FileSystemAccessRule($identity, 'FullControl', 'ContainerInherit,ObjectInherit', 'None', 'Allow')
    $security.AddAccessRule($rule)
}
if (-not (Get-Acl -LiteralPath $destination).AreAccessRulesProtected) {
    Set-Acl -LiteralPath $destination -AclObject $security
}
Copy-Item -LiteralPath $binarySource -Destination $binaryTarget -Force
if ($tokenSource -ne $tokenTarget) { Copy-Item -LiteralPath $tokenSource -Destination $tokenTarget -Force }
$fileSecurity = New-Object System.Security.AccessControl.FileSecurity
$fileSecurity.SetAccessRuleProtection($true, $false)
foreach ($identity in @($user, $system)) {
    $fileSecurity.AddAccessRule((New-Object System.Security.AccessControl.FileSystemAccessRule($identity, 'FullControl', 'Allow')))
}
if (-not (Get-Acl -LiteralPath $tokenTarget).AreAccessRulesProtected) {
    Set-Acl -LiteralPath $tokenTarget -AclObject $fileSecurity
}

$logFile = Join-Path $destination 'agent.log'
$arguments = "-node-id $NodeId -token-file `"$tokenTarget`" -log-file `"$logFile`""
$runKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
New-Item -Path $runKey -Force | Out-Null
New-ItemProperty -Path $runKey -Name 'oxnetwork-host-agent' -PropertyType String -Value "`"$binaryTarget`" $arguments" -Force | Out-Null
$oldTask = Get-ScheduledTask -TaskName 'oxnetwork-host-agent' -ErrorAction SilentlyContinue
if ($oldTask) { Unregister-ScheduledTask -TaskName 'oxnetwork-host-agent' -Confirm:$false }
Start-Process -FilePath $binaryTarget -ArgumentList $arguments -WindowStyle Hidden
Write-Output "Installed oxnetwork-host-agent for $NodeId; it starts at this user's logon."
