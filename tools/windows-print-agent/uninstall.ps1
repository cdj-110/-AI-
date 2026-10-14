$installDir = Join-Path $env:LOCALAPPDATA "Weikong\PrintAgent"
$startupFile = Join-Path ([Environment]::GetFolderPath("Startup")) "Weikong Print Agent.cmd"
Remove-Item -LiteralPath $startupFile -Force -ErrorAction SilentlyContinue
Get-CimInstance Win32_Process -Filter "Name = 'powershell.exe'" | Where-Object { $_.CommandLine -like "*$installDir*print-agent.ps1*" } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force }
Remove-Item -LiteralPath $installDir -Recurse -Force -ErrorAction SilentlyContinue
Write-Host "Weikong print agent uninstalled." -ForegroundColor Green
