param(
  [string]$Python = "$PSScriptRoot/.venv/Scripts/python.exe",
  [string]$Config = "$PSScriptRoot/config.local.json",
  [int]$Port = 18741
)
$ErrorActionPreference = 'Stop'
if (-not (Test-Path -LiteralPath $Python)) { throw 'Create the Python environment and install requirements.lock.txt first.' }
if (-not (Test-Path -LiteralPath $Config)) { throw 'Copy config.example.json to config.local.json and configure the isolated paths first.' }
& $Python "$PSScriptRoot/team.py" --config $Config doctor
if ($LASTEXITCODE -ne 0) { throw 'Codex login or isolated repository is unavailable.' }
Write-Host "Local task status: http://127.0.0.1:$Port"
& $Python "$PSScriptRoot/team.py" --config $Config serve --port $Port
exit $LASTEXITCODE
