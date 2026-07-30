param(
  [string]$Config = "config.local.json",
  [string]$Listen = ""
)
$ErrorActionPreference = "Stop"
$python = Join-Path $PSScriptRoot ".venv\Scripts\python.exe"
if (-not (Test-Path $python)) {
  throw "Python虚拟环境不存在，请先执行 install.ps1"
}
if (-not (Test-Path (Join-Path $PSScriptRoot $Config))) {
  Copy-Item (Join-Path $PSScriptRoot "config.example.json") (Join-Path $PSScriptRoot $Config)
}
$arguments = @("-m", "gateway.main", "--config", $Config)
if ($Listen) { $arguments += @("--listen", $Listen) }
& $python @arguments
