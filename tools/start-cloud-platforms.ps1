param(
  [switch]$SkipDocker
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

function Test-TcpPort {
  param([string]$HostName, [int]$Port)
  $client = [System.Net.Sockets.TcpClient]::new()
  try {
    $task = $client.ConnectAsync($HostName, $Port)
    if (-not $task.Wait(800)) { return $false }
    return $client.Connected
  } catch {
    return $false
  } finally {
    $client.Dispose()
  }
}

function Wait-TcpPort {
  param(
    [string]$Name,
    [string]$HostName = "127.0.0.1",
    [int]$Port,
    [int]$TimeoutSeconds = 60
  )
  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  do {
    if (Test-TcpPort -HostName $HostName -Port $Port) {
      Write-Host "  OK  $Name ($HostName`:$Port)" -ForegroundColor Green
      return
    }
    Start-Sleep -Milliseconds 800
  } while ((Get-Date) -lt $deadline)
  throw "$Name did not become ready at $HostName`:$Port within $TimeoutSeconds seconds."
}

function Start-NpmScript {
  param(
    [string]$WorkingDirectory,
    [string]$ScriptName,
    [string]$OutputLog,
    [string]$ErrorLog,
    [hashtable]$Environment = @{}
  )
  $savedEnvironment = @{}
  foreach ($name in $Environment.Keys) {
    $savedEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, "Process")
    [Environment]::SetEnvironmentVariable($name, [string]$Environment[$name], "Process")
  }
  try {
    Start-Process `
      -FilePath "npm.cmd" `
      -ArgumentList @("run", $ScriptName) `
      -WorkingDirectory $WorkingDirectory `
      -RedirectStandardOutput $OutputLog `
      -RedirectStandardError $ErrorLog `
      -WindowStyle Hidden | Out-Null
  } finally {
    foreach ($name in $Environment.Keys) {
      [Environment]::SetEnvironmentVariable($name, $savedEnvironment[$name], "Process")
    }
  }
}

Write-Host "Starting Weikong IoT cloud platforms..." -ForegroundColor Cyan
Write-Host "Project: $root"

Push-Location $root
try {
  if (-not $SkipDocker) {
    Write-Host "Starting Docker dependencies: postgres, timescaledb, redis, emqx"
    docker compose up -d --wait --wait-timeout 120 postgres timescaledb redis emqx
    if ($LASTEXITCODE -ne 0) { throw "Docker dependencies failed to start. Make sure Docker Desktop is running." }
  }

  Write-Host "Waiting for infrastructure..." -ForegroundColor Cyan
  Wait-TcpPort -Name "PostgreSQL" -Port 5432 -TimeoutSeconds 90
  Wait-TcpPort -Name "TimescaleDB" -Port 5433 -TimeoutSeconds 90
  Wait-TcpPort -Name "Redis" -Port 6379 -TimeoutSeconds 60
  Wait-TcpPort -Name "EMQX" -Port 1883 -TimeoutSeconds 90

  $apiDir = Join-Path $root "apps\api"
  $ingestDir = Join-Path $root "apps\ingest"
  $webDir = Join-Path $root "apps\web"

  Write-Host "Applying database migrations..." -ForegroundColor Cyan
  Push-Location $apiDir
  try {
    & npx.cmd prisma migrate deploy
    if ($LASTEXITCODE -ne 0) { throw "Database migration failed." }
  } finally {
    Pop-Location
  }

  if (-not (Test-TcpPort -HostName "127.0.0.1" -Port 3100)) {
    Write-Host "Starting API on http://127.0.0.1:3100"
    $apiOutputLog = Join-Path $apiDir "api-runtime.out.log"
    $apiErrorLog = Join-Path $apiDir "api-runtime.err.log"
    Remove-Item -LiteralPath $apiOutputLog, $apiErrorLog -Force -ErrorAction SilentlyContinue
    Start-NpmScript `
      -WorkingDirectory $apiDir `
      -ScriptName "start:dev" `
      -OutputLog $apiOutputLog `
      -ErrorLog $apiErrorLog
  } else {
    Write-Host "API is already running on port 3100." -ForegroundColor Yellow
  }

  try {
    Wait-TcpPort -Name "API" -Port 3100 -TimeoutSeconds 45
  } catch {
    Write-Host "API failed to start. Recent error log:" -ForegroundColor Red
    $apiErrorLog = Join-Path $apiDir "api-runtime.err.log"
    if (Test-Path $apiErrorLog) { Get-Content -Encoding utf8 $apiErrorLog -Tail 35 }
    throw
  }

  # Do not run a second worker against the same platform: old workers can overwrite
  # newer topology capabilities even though their telemetry writes are deduplicated.
  $taskIngestAlreadyRunning = $false
  if (Get-Command docker -ErrorAction SilentlyContinue) {
    $taskIngestContainer = (& docker compose ps -q ingest 2>$null)
    if ($LASTEXITCODE -eq 0 -and $taskIngestContainer) {
      $taskIngestAlreadyRunning = ((& docker inspect --format '{{.State.Running}}' $taskIngestContainer 2>$null) -eq 'true')
    }
  }
  if (-not $taskIngestAlreadyRunning) {
    $taskIngestPath = Join-Path $ingestDir 'node_modules'
    $taskIngestAlreadyRunning = [bool](Get-CimInstance Win32_Process -Filter "Name='node.exe'" | Where-Object {
      $_.CommandLine -like "*$taskIngestPath*ts-node*src/main.ts*"
    })
  }
  if ($taskIngestAlreadyRunning) {
    Write-Host "Ingest is already running; duplicate worker skipped." -ForegroundColor Yellow
  } else {
  Write-Host "Starting ingest worker for MQTT telemetry and heartbeat"
  $ingestOutputLog = Join-Path $ingestDir "ingest-runtime.out.log"
  $ingestErrorLog = Join-Path $ingestDir "ingest-runtime.err.log"
  Remove-Item -LiteralPath $ingestOutputLog, $ingestErrorLog -Force -ErrorAction SilentlyContinue
  Start-NpmScript `
    -WorkingDirectory $ingestDir `
    -ScriptName "start:dev" `
    -OutputLog $ingestOutputLog `
    -ErrorLog $ingestErrorLog `
    -Environment @{
      MQTT_URL = "mqtt://localhost:1883"
      MQTT_USERNAME = "platform-ingest"
      MQTT_INGEST_PASSWORD = $env:MQTT_INGEST_PASSWORD
      DATABASE_URL = "postgresql://weikong@localhost:5432/weikong_iot?schema=public"
      TIMESCALE_DATABASE_URL = "postgresql://weikong@localhost:5433/weikong_ts"
    }
  }

  if (-not (Test-TcpPort -HostName "127.0.0.1" -Port 5174)) {
    Write-Host "Starting tenant IoT platform on http://127.0.0.1:5174"
    Start-NpmScript `
      -WorkingDirectory $webDir `
      -ScriptName "dev:tenant" `
      -OutputLog (Join-Path $root ".runtime-web.log") `
      -ErrorLog (Join-Path $root ".runtime-web.err.log")
  } else {
    Write-Host "Tenant platform is already running on port 5174." -ForegroundColor Yellow
  }

  if (-not (Test-TcpPort -HostName "127.0.0.1" -Port 5176)) {
    Write-Host "Starting admin platform on http://127.0.0.1:5176"
    Start-NpmScript `
      -WorkingDirectory $webDir `
      -ScriptName "dev:admin" `
      -OutputLog (Join-Path $root ".runtime-admin-web.log") `
      -ErrorLog (Join-Path $root ".runtime-admin-web.err.log")
  } else {
    Write-Host "Admin platform is already running on port 5176." -ForegroundColor Yellow
  }

  Wait-TcpPort -Name "Tenant platform" -Port 5174 -TimeoutSeconds 30
  Wait-TcpPort -Name "Admin platform" -Port 5176 -TimeoutSeconds 30

  Write-Host ""
  Write-Host "All services are ready." -ForegroundColor Green
  Write-Host "Tenant IoT platform: http://127.0.0.1:5174"
  Write-Host "Admin platform:      http://127.0.0.1:5176"
  Write-Host "API:                 http://127.0.0.1:3100"
  Write-Host "API error log:       apps\api\api-runtime.err.log"
  Write-Host "Ingest logs:         apps\ingest\ingest-runtime.out.log"
} finally {
  Pop-Location
}
