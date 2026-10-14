param(
  [string]$PrinterName = "",
  [int]$Port = 17896,
  [string[]]$AllowedOrigin = @("http://127.0.0.1:5176", "http://localhost:5176"),
  [string]$CloudApiUrl = $null,
  [string]$CloudAgentToken = $null,
  [switch]$RotatePairingKey
)

$ErrorActionPreference = "Stop"
$installDir = Join-Path $env:LOCALAPPDATA "Weikong\PrintAgent"
New-Item -ItemType Directory -Path $installDir -Force | Out-Null
Get-CimInstance Win32_Process -Filter "Name = 'powershell.exe'" -ErrorAction SilentlyContinue |
  Where-Object { $_.CommandLine -like "*$installDir*print-agent.ps1*" } |
  ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
Start-Sleep -Milliseconds 500
# Windows PowerShell 5 treats UTF-8 files without BOM as the system code page.
# Rewrite the script explicitly so Chinese diagnostics cannot corrupt parsing.
Get-Content -LiteralPath (Join-Path $PSScriptRoot "print-agent.ps1") -Raw -Encoding utf8 |
  Set-Content -LiteralPath (Join-Path $installDir "print-agent.ps1") -Encoding utf8
Copy-Item -LiteralPath (Join-Path $PSScriptRoot "WeikongLabelPrinter.cs") -Destination $installDir -Force

if (-not $PrinterName) {
  Add-Type -AssemblyName System.Drawing
  $printers = @([Drawing.Printing.PrinterSettings]::InstalledPrinters)
  $PrinterName = [string]($printers | Where-Object { $_ -match '^Godex G500 GZPL$' } | Select-Object -First 1)
  if (-not $PrinterName) { $PrinterName = [string]($printers | Where-Object { $_ -match 'Godex|GoDEX|G500' } | Select-Object -First 1) }
  if (-not $PrinterName) { $PrinterName = [string]($printers | Select-Object -First 1) }
}

$existingConfigPath = Join-Path $installDir "config.json"
$existingConfig = if (Test-Path -LiteralPath $existingConfigPath) { Get-Content -LiteralPath $existingConfigPath -Raw -Encoding utf8 | ConvertFrom-Json } else { $null }
if ($existingConfig -and $existingConfig.pairingKey -and -not $RotatePairingKey) {
  $pairingKey = [string]$existingConfig.pairingKey
} else {
  $keyBytes = New-Object byte[] 24
  [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($keyBytes)
  $pairingKey = [Convert]::ToBase64String($keyBytes)
}
$effectiveCloudApiUrl = if ($null -ne $CloudApiUrl) { $CloudApiUrl.TrimEnd('/') } elseif ($existingConfig) { [string]$existingConfig.cloudApiUrl } else { "" }
$effectiveCloudAgentToken = if ($null -ne $CloudAgentToken) { $CloudAgentToken } elseif ($existingConfig) { [string]$existingConfig.cloudAgentToken } else { "" }
$config = @{
  port = $Port
  defaultPrinter = $PrinterName
  pairingKey = $pairingKey
  allowedOrigins = @($AllowedOrigin)
  cloudApiUrl = $effectiveCloudApiUrl
  cloudAgentToken = $effectiveCloudAgentToken
}
$config | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $installDir "config.json") -Encoding utf8

$launcher = Join-Path $installDir "start-print-agent.cmd"
"@echo off`r`npowershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$installDir\print-agent.ps1`"" | Set-Content -LiteralPath $launcher -Encoding ascii
$startup = [Environment]::GetFolderPath("Startup")
Copy-Item -LiteralPath $launcher -Destination (Join-Path $startup "Weikong Print Agent.cmd") -Force
Start-Process -FilePath $launcher -WindowStyle Hidden

Write-Host ""; Write-Host "Weikong print agent installed." -ForegroundColor Green
Write-Host "Printer: $PrinterName"
Write-Host "Agent URL: http://127.0.0.1:$Port"
Write-Host "Pairing key (paste into admin platform):" -ForegroundColor Yellow
Write-Host $pairingKey -ForegroundColor Cyan
Write-Host "Config: $installDir\config.json"
