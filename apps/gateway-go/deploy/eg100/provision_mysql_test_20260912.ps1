$ErrorActionPreference = 'Stop'
$container = 'eg100-mysql-integration'
$volume = 'eg100-mysql-integration-data'
$artifactDir = Join-Path (Get-Location) 'build/mysql-integration-20260912'
New-Item -ItemType Directory -Path $artifactDir -Force | Out-Null
if ((& docker ps -a --format '{{.Names}}') -contains $container) { throw 'Test container already exists; inspect rather than overwrite.' }
if ((& docker volume ls --format '{{.Name}}') -contains $volume) { throw 'Test data volume already exists; inspect rather than overwrite.' }
if (Get-NetTCPConnection -LocalPort 3306 -State Listen -ErrorAction SilentlyContinue) { throw 'Port 3306 already in use.' }
$credentials = @{}
foreach ($account in @('root','eg100_writer','eg100_tester')) {
    $path = Join-Path $artifactDir ($account + '.credential.clixml')
    if (Test-Path -LiteralPath $path) { throw 'Credential exists; inspect before retry.' }
    $bytes = New-Object byte[] 30
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    $rng.GetBytes($bytes)
    $rng.Dispose()
    $plain = [Convert]::ToBase64String($bytes)
    $credential = [pscredential]::new($account, (ConvertTo-SecureString $plain -AsPlainText -Force))
    $credential | Export-Clixml -LiteralPath $path
    $credentials[$account] = $plain
}
try {
    # Pass the secret by inherited environment, never print it or put it in the command line.
    $env:MYSQL_ROOT_PASSWORD = $credentials['root']
    & docker run -d --name $container --label purpose=eg100-mysql-integration --memory 768m --cpus 1 -p 127.0.0.1:3306:3306 -v "${volume}:/var/lib/mysql" -e MYSQL_ROOT_PASSWORD -e MYSQL_ROOT_HOST=localhost mysql@sha256:85b9bf2e29cf836ecb8c2a15a935d4ba0c606631dff1dd79531a11983c638f2a --character-set-server=utf8mb4 --collation-server=utf8mb4_bin --max-connections=40 --innodb-buffer-pool-size=134217728
    if ($LASTEXITCODE -ne 0) { throw 'Container creation failed; credentials retained.' }
    $env:MYSQL_PWD = $credentials['root']
    $ready = $false
    for ($i=0; $i -lt 60; $i++) {
        $check = & docker exec -e MYSQL_PWD $container mysql -uroot --batch --skip-column-names -e 'SELECT 1;' 2>$null
        if ($LASTEXITCODE -eq 0 -and $check -eq '1') { $ready = $true; break }
        Start-Sleep -Seconds 2
    }
    if (!$ready) { throw 'MySQL did not become ready.' }
    $writer = $credentials['eg100_writer']
    $tester = $credentials['eg100_tester']
    $sql = @"
CREATE DATABASE eg100_integration CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;
CREATE USER 'eg100_writer'@'%' IDENTIFIED BY '$writer' WITH MAX_USER_CONNECTIONS 8;
GRANT SELECT,INSERT,UPDATE,CREATE ON eg100_integration.* TO 'eg100_writer'@'%';
CREATE USER 'eg100_tester'@'%' IDENTIFIED BY '$tester' WITH MAX_USER_CONNECTIONS 2;
GRANT SELECT,INSERT,UPDATE,CREATE,DROP ON eg100_integration.* TO 'eg100_tester'@'%';
SELECT VERSION();
SHOW GRANTS FOR 'eg100_writer'@'%';
"@
    $sql | & docker exec -i -e MYSQL_PWD $container mysql -uroot --batch --skip-column-names
    if ($LASTEXITCODE -ne 0) { throw 'Test database provisioning failed; inspect retained container and credentials.' }
    Write-Output 'MySQL test database ready. Passwords stored using Windows DPAPI; root remains local to container.'
} finally {
    Remove-Item Env:MYSQL_ROOT_PASSWORD -ErrorAction SilentlyContinue
    Remove-Item Env:MYSQL_PWD -ErrorAction SilentlyContinue
}
