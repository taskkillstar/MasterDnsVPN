# ==============================================================================
# sync_upstream.ps1 - PowerShell entrypoint for syncing upstream changes
# ==============================================================================
[CmdletBinding()]
param(
    [string]$Ref = "upstream/main"
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$BashScript = Join-Path $ScriptDir "sync_upstream.sh" -Resolve

# Locate Git for Windows bash.exe (avoiding WSL shim in System32)
$GitBash = "$env:LOCALAPPDATA\Programs\Git\bin\bash.exe"
if (-not (Test-Path $GitBash)) {
    $GitBash = "C:\Program Files\Git\bin\bash.exe"
}
if (-not (Test-Path $GitBash)) {
    # Fallback to git-bash in PATH
    $GitBash = (Get-Command bash.exe -ErrorAction SilentlyContinue | Where-Object { $_.Source -notlike "*System32*" } | Select-Object -ExpandProperty Source -First 1)
}

if (-not $GitBash -or -not (Test-Path $GitBash)) {
    Write-Error "Could not locate Git for Windows bash.exe. Please ensure Git for Windows is installed."
    exit 1
}

& $GitBash $BashScript $Ref
exit $LASTEXITCODE
