<#
  Read-only diagnostics for the oxnet.work Windows host agent.
  Run it in a normal (non-elevated) PowerShell window as the user the agent runs as:

      powershell -NoProfile -ExecutionPolicy Bypass -File .\diagnose-windows-agent.ps1

  It changes nothing and never prints the token. It answers: did the agent start at
  sign-in, is its startup entry present and enabled, is it running now, and can it
  reach oxnet.work? Paste the output into the task board or hand it to Codex.
  Run it from a normal desktop PowerShell, not from inside a packaged app such as the
  Claude desktop app, whose shell sees a redirected view of AppData and the registry.
#>
$ErrorActionPreference = 'Continue'
$agentDir = Join-Path $env:LOCALAPPDATA 'oxnetwork\agent'
$runName  = 'oxnetwork-host-agent'

function Section($title) { Write-Output ''; Write-Output "== $title" }

Section 'Machine and session'
$os = Get-CimInstance Win32_OperatingSystem
Write-Output ("computer:      {0}" -f $env:COMPUTERNAME)
Write-Output ("user:          {0}\{1}" -f $env:USERDOMAIN, $env:USERNAME)
Write-Output ("last boot:     {0}" -f $os.LastBootUpTime)
$explorer = Get-Process explorer -ErrorAction SilentlyContinue | Sort-Object StartTime | Select-Object -First 1
if ($explorer) { Write-Output ("sign-in shell: {0} (explorer.exe start)" -f $explorer.StartTime) }
Write-Output ("now:           {0}" -f (Get-Date))
Write-Output ("fast startup:  HiberbootEnabled = {0}" -f ((Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Power' -ErrorAction SilentlyContinue).HiberbootEnabled))

Section 'Agent files'
if (Test-Path $agentDir) {
    Get-ChildItem $agentDir | Select-Object Name, Length, LastWriteTime | Format-Table -AutoSize | Out-String -Width 200 | Write-Output
} else {
    Write-Output "MISSING: $agentDir"
}

Section 'Startup entry (HKCU Run)'
$run = Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' -ErrorAction SilentlyContinue
$value = $null
if ($run) { $value = $run.$runName }
if ($value) {
    # Show the command but never any token value.
    Write-Output ("{0} = {1}" -f $runName, $value)
    $exe = ($value -replace '^\s*"([^"]+)".*$', '$1')
    Write-Output ("exe exists: {0}" -f (Test-Path $exe))
} else {
    Write-Output "NO Run entry named $runName (the agent will not start at sign-in)"
}
$approved = Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run' -ErrorAction SilentlyContinue
if ($approved -and $approved.$runName) {
    $first = $approved.$runName[0]
    $state = if ($first -eq 2 -or $first -eq 6) { 'enabled' } elseif ($first -eq 3 -or $first -eq 7) { 'DISABLED in Task Manager > Startup apps' } else { "unknown ($first)" }
    Write-Output ("Startup apps state: {0}" -f $state)
} else {
    Write-Output 'Startup apps state: no override recorded (enabled by default)'
}

Section 'Scheduled tasks with oxnet in the name'
$tasks = Get-ScheduledTask -ErrorAction SilentlyContinue | Where-Object { $_.TaskName -match 'oxnet' }
if ($tasks) { $tasks | ForEach-Object { Write-Output ("{0}  state={1}" -f $_.TaskName, $_.State) } } else { Write-Output 'none' }

Section 'Running now'
$procs = Get-CimInstance Win32_Process -Filter "Name = 'oxnet-host-agent.exe'" -ErrorAction SilentlyContinue
if ($procs) {
    foreach ($p in $procs) {
        $cmd = ($p.CommandLine -replace '(-token-file\s+"?)[^"]+("?)', '$1<hidden>$2')
        Write-Output ("pid {0} started {1}" -f $p.ProcessId, $p.CreationDate)
        Write-Output ("  {0}" -f $cmd)
        if ($explorer -and $p.CreationDate -gt $explorer.StartTime.AddMinutes(3)) {
            Write-Output ("  NOTE: started {0:N0} minutes after sign-in, so it was not started by the sign-in entry" -f (($p.CreationDate - $explorer.StartTime).TotalMinutes))
        }
    }
} else {
    Write-Output 'NOT RUNNING'
}

Section 'Last 25 log lines'
$log = Join-Path $agentDir 'agent.log'
if (Test-Path $log) { Get-Content $log -Tail 25 } else { 'no agent.log' }

Section 'Windows events mentioning the agent (last 3 days)'
try {
    $events = Get-WinEvent -FilterHashtable @{ LogName = 'Application'; StartTime = (Get-Date).AddDays(-3) } -ErrorAction Stop |
        Where-Object { $_.Message -match 'oxnet-host-agent' } | Select-Object -First 5
    if ($events) { $events | Select-Object TimeCreated, Id, ProviderName | Format-Table -AutoSize | Out-String -Width 200 | Write-Output } else { 'none' }
} catch { 'could not read the Application log' }

Section 'Reachability'
foreach ($url in 'https://oxnet.work/readyz') {
    try { $r = Invoke-WebRequest -Uri $url -UseBasicParsing -TimeoutSec 8 -ErrorAction Stop; Write-Output ("{0} -> {1}" -f $url, $r.StatusCode) }
    catch { Write-Output ("{0} -> FAILED: {1}" -f $url, $_.Exception.Message) }
}
$ts = Get-Command tailscale -ErrorAction SilentlyContinue
if ($ts) { Write-Output ('tailscale: ' + ((& tailscale status 2>&1 | Select-Object -First 1))) } else { Write-Output 'tailscale CLI not on PATH' }

Write-Output ''
Write-Output 'Reading the result: if the Run entry exists, is enabled and the exe exists, but the agent started long after'
Write-Output 'sign-in (see NOTE above), the entry is not firing or the process exits at once; check agent.log. If the Run entry'
Write-Output 'is missing or DISABLED, re-run the installer (deploy/agents/install-windows.ps1) or enable it in Task Manager.'
