param([string]$ConfigPath = (Join-Path $PSScriptRoot "config.json"))

$ErrorActionPreference = "Stop"
$agentVersion = "1.0.0"
if (-not (Test-Path -LiteralPath $ConfigPath)) { throw "Print agent config not found: $ConfigPath. Run install.ps1 first." }
$config = Get-Content -LiteralPath $ConfigPath -Raw -Encoding utf8 | ConvertFrom-Json
$port = if ($config.port) { [int]$config.port } else { 17896 }
$allowedOrigins = @($config.allowedOrigins)
$cloudApiUrl = [string]$config.cloudApiUrl
$cloudAgentToken = [string]$config.cloudAgentToken

Add-Type -AssemblyName System.Drawing
Add-Type -AssemblyName System.Windows.Forms
Add-Type -TypeDefinition (Get-Content -LiteralPath (Join-Path $PSScriptRoot "WeikongLabelPrinter.cs") -Raw -Encoding utf8) -ReferencedAssemblies System.Drawing,System.Windows.Forms

function Write-JsonResponse($context, [int]$status, $body, [string]$origin = "") {
  $json = $body | ConvertTo-Json -Depth 10 -Compress
  $bytes = [Text.Encoding]::UTF8.GetBytes($json)
  $context.Response.StatusCode = $status
  $context.Response.ContentType = "application/json; charset=utf-8"
  if ($origin) { $context.Response.Headers["Access-Control-Allow-Origin"] = $origin }
  $context.Response.Headers["Access-Control-Allow-Headers"] = "Content-Type, X-Weikong-Print-Key"
  $context.Response.Headers["Access-Control-Allow-Methods"] = "GET, POST, OPTIONS"
  $context.Response.Headers["Access-Control-Allow-Private-Network"] = "true"
  $context.Response.Headers["Cache-Control"] = "no-store"
  $context.Response.ContentLength64 = $bytes.Length
  $context.Response.OutputStream.Write($bytes, 0, $bytes.Length)
  $context.Response.Close()
}

function Get-RequestBody($request) {
  $reader = [IO.StreamReader]::new($request.InputStream, $request.ContentEncoding)
  try { return $reader.ReadToEnd() | ConvertFrom-Json } finally { $reader.Dispose() }
}

function Is-OriginAllowed([string]$origin) {
  if (-not $origin) { return $true }
  return $allowedOrigins -contains $origin
}

function Invoke-CloudRequest([string]$path, [string]$method = "GET", $body = $null) {
  $headers = @{ "X-Print-Agent-Token" = $cloudAgentToken }
  $arguments = @{ Uri = "$($cloudApiUrl.TrimEnd('/'))$path"; Method = $method; Headers = $headers; UseBasicParsing = $true; TimeoutSec = 20 }
  if ($null -ne $body) {
    $arguments.ContentType = "application/json; charset=utf-8"
    $arguments.Body = ($body | ConvertTo-Json -Depth 8 -Compress)
  }
  $response = Invoke-WebRequest @arguments
  return $response.Content | ConvertFrom-Json
}

function Start-CloudWorker {
  if (-not $cloudApiUrl -or -not $cloudAgentToken) { return }
  $workerScript = {
    param($apiUrl, $token, $configFile)
    $workerConfig = Get-Content -LiteralPath $configFile -Raw -Encoding utf8 | ConvertFrom-Json
    Add-Type -AssemblyName System.Drawing
    Add-Type -AssemblyName System.Windows.Forms
    Add-Type -TypeDefinition (Get-Content -LiteralPath (Join-Path (Split-Path $configFile) "WeikongLabelPrinter.cs") -Raw -Encoding utf8) -ReferencedAssemblies System.Drawing,System.Windows.Forms
    while ($true) {
      try {
        $headers = @{ "X-Print-Agent-Token" = $token }
        $claimResponse = Invoke-RestMethod -Uri "$($apiUrl.TrimEnd('/'))/api/label-print/agent/claim" -Method Get -Headers $headers -TimeoutSec 25
        # The cloud API uses a common { code, message, data } envelope.
        # Keep compatibility with both enveloped and direct responses.
        $claim = if ($claimResponse.PSObject.Properties['data']) { $claimResponse.data } else { $claimResponse }
        if ($claim.job) {
          $job = $claim.job
          try {
            $size = ([string]$job.labelSize).Split('x')
            $base64 = [string]$job.qrDataUrl
            if ($base64 -match '^data:image/[^;]+;base64,(.+)$') { $base64 = $Matches[1] }
            # The Windows machine is the source of truth for the installed
            # driver name. A cloud terminal may retain an older display name.
            $printer = [string]$workerConfig.defaultPrinter
            $item = [WeikongLabelItem]@{ Sn = [string]$job.sn; BindCode = [string]$job.bindCode; QrBase64 = $base64 }
            [WeikongLabelPrinter]::Print($printer, [double]$size[0], [double]$size[1], [int]$job.copies, @($item))
            $result = @{ success = $true; printerName = $printer }
          } catch {
            $result = @{ success = $false; printerName = [string]$workerConfig.defaultPrinter; error = $_.Exception.Message }
          }
          Invoke-RestMethod -Uri "$($apiUrl.TrimEnd('/'))/api/label-print/agent/jobs/$($job.id)/complete" -Method Post -Headers $headers -ContentType "application/json; charset=utf-8" -Body ($result | ConvertTo-Json -Compress) -TimeoutSec 20 | Out-Null
        }
      } catch {
        # Network/API errors are expected while the cloud platform is offline.
      }
      Start-Sleep -Seconds 3
    }
  }
  Start-Job -ScriptBlock $workerScript -ArgumentList $cloudApiUrl, $cloudAgentToken, $ConfigPath | Out-Null
  Write-Host "Cloud print worker connected to $cloudApiUrl" -ForegroundColor Green
}

$listener = [Net.HttpListener]::new()
$listener.Prefixes.Add("http://127.0.0.1:$port/")
$listener.Start()
Write-Host "Weikong print agent $agentVersion is listening at http://127.0.0.1:$port" -ForegroundColor Green
Write-Host "Default printer: $($config.defaultPrinter)"
Start-CloudWorker

try {
  while ($listener.IsListening) {
    $context = $listener.GetContext()
    $request = $context.Request
    $origin = [string]$request.Headers["Origin"]
    if (-not (Is-OriginAllowed $origin)) {
      Write-JsonResponse $context 403 @{ ok = $false; message = "Origin is not allowed" }
      continue
    }
    if ($request.HttpMethod -eq "OPTIONS") {
      Write-JsonResponse $context 204 @{} $origin
      continue
    }

    try {
      $path = $request.Url.AbsolutePath.TrimEnd('/')
      if ($path -eq "/api/health" -and $request.HttpMethod -eq "GET") {
        Write-JsonResponse $context 200 @{ ok = $true; version = $agentVersion; defaultPrinter = $config.defaultPrinter } $origin
        continue
      }
      if ([string]$request.Headers["X-Weikong-Print-Key"] -ne [string]$config.pairingKey) {
        Write-JsonResponse $context 401 @{ ok = $false; message = "打印服务配对密钥不正确" } $origin
        continue
      }
      if ($path -eq "/api/printers" -and $request.HttpMethod -eq "GET") {
        Write-JsonResponse $context 200 @{ ok = $true; printers = [WeikongLabelPrinter]::GetPrinters(); defaultPrinter = $config.defaultPrinter } $origin
        continue
      }
      if ($path -eq "/api/print" -and $request.HttpMethod -eq "POST") {
        $body = Get-RequestBody $request
        $printer = if ($body.printer) { [string]$body.printer } else { [string]$config.defaultPrinter }
        if (-not $printer) { throw "未配置默认打印机" }
        $items = @($body.items | ForEach-Object {
          $base64 = [string]$_.qrDataUrl
          if ($base64 -match '^data:image/[^;]+;base64,(.+)$') { $base64 = $Matches[1] }
          [WeikongLabelItem]@{ Sn = [string]$_.sn; BindCode = [string]$_.bindCode; QrBase64 = $base64 }
        })
        [WeikongLabelPrinter]::Print($printer, [double]$body.widthMm, [double]$body.heightMm, [int]$body.copies, $items)
        Write-JsonResponse $context 200 @{ ok = $true; printer = $printer; labels = $items.Count; copies = [int]$body.copies } $origin
        continue
      }
      Write-JsonResponse $context 404 @{ ok = $false; message = "Not found" } $origin
    } catch {
      Write-JsonResponse $context 500 @{ ok = $false; message = $_.Exception.Message } $origin
    }
  }
} finally {
  $listener.Stop()
  $listener.Close()
}
