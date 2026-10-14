$ErrorActionPreference='Stop'
$container='eg100-sqlserver-integration'
$volume='eg100-sqlserver-integration-data'
$dir=Join-Path (Get-Location) 'build/sqlserver-integration-20260912'
if ((& docker ps -a --format '{{.Names}}') -contains $container) { throw 'Container already exists; do not overwrite.' }
if ((& docker volume ls --format '{{.Name}}') -contains $volume) { throw 'Data volume already exists; do not overwrite.' }
if (Get-NetTCPConnection -LocalPort 1433 -State Listen -ErrorAction SilentlyContinue) { throw 'Port is occupied.' }
New-Item -ItemType Directory -Path $dir -Force | Out-Null
$secrets=@{}
foreach($account in @('sa','eg100_writer','eg100_tester')) {
    $path=Join-Path $dir ($account+'.credential.clixml')
    if (Test-Path -LiteralPath $path) { throw 'Credentials exist; inspect before retry.' }
    $bytes=New-Object byte[] 30
    $rng=[Security.Cryptography.RandomNumberGenerator]::Create()
    $rng.GetBytes($bytes)
    $rng.Dispose()
    $plain='T9!a'+[Convert]::ToBase64String($bytes)
    [pscredential]::new($account,(ConvertTo-SecureString $plain -AsPlainText -Force)) | Export-Clixml -LiteralPath $path
    $secrets[$account]=$plain
}
$image=(& docker image inspect mcr.microsoft.com/mssql/server:2022-latest --format '{{index .RepoDigests 0}}').Trim()
if ($LASTEXITCODE -ne 0 -or !$image.StartsWith('mcr.microsoft.com/mssql/server@sha256:')) { throw 'Official image not available.' }
try {
    $env:MSSQL_SA_PASSWORD=$secrets['sa']
    & docker run -d --name $container --label purpose=eg100-sqlserver-integration --memory 3g --cpus 2 -e ACCEPT_EULA=Y -e MSSQL_PID=Developer -e MSSQL_MEMORY_LIMIT_MB=2048 -e MSSQL_SA_PASSWORD -p 127.0.0.1:1433:1433 -p 192.168.1.141:1433:1433 -v "${volume}:/var/opt/mssql" $image
    if ($LASTEXITCODE -ne 0) { throw 'Container creation failed.' }
    $env:SQLCMDPASSWORD=$secrets['sa']
    $ready=$false
    for($i=0;$i -lt 60;$i++) {
        $result=& docker exec -e SQLCMDPASSWORD $container /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -C -b -l 3 -h -1 -W -Q 'SET NOCOUNT ON; SELECT 1;' 2>$null
        if ($LASTEXITCODE -eq 0 -and ($result -join '').Trim() -eq '1') { $ready=$true; break }
        Start-Sleep -Seconds 2
    }
    if (!$ready) { throw 'SQL Server not ready.' }
    $writer=$secrets['eg100_writer']
    $tester=$secrets['eg100_tester']
    $sql=@"
SET NOCOUNT ON;
CREATE DATABASE eg100_integration;
GO
ALTER DATABASE eg100_integration SET RECOVERY SIMPLE;
CREATE LOGIN eg100_writer WITH PASSWORD=N'$writer', CHECK_POLICY=ON, DEFAULT_DATABASE=eg100_integration;
CREATE LOGIN eg100_tester WITH PASSWORD=N'$tester', CHECK_POLICY=ON, DEFAULT_DATABASE=eg100_integration;
GO
USE eg100_integration;
CREATE USER eg100_writer FOR LOGIN eg100_writer;
CREATE USER eg100_tester FOR LOGIN eg100_tester;
CREATE TABLE dbo.eg100_samples (
 record_id VARCHAR(64) NOT NULL PRIMARY KEY,
 gateway_key NVARCHAR(256) NOT NULL, device_key NVARCHAR(256) NOT NULL, metric NVARCHAR(256) NOT NULL,
 sampled_ms BIGINT NOT NULL, received_ms BIGINT, source_ms BIGINT,
 value_type VARCHAR(24) NOT NULL, value_json NVARCHAR(MAX) NOT NULL, integer_value VARCHAR(32),
 float_value FLOAT, boolean_value SMALLINT, text_value NVARCHAR(MAX), actual_type NVARCHAR(128),
 quality VARCHAR(24), unit NVARCHAR(128), diagnostic NVARCHAR(MAX)
);
GRANT SELECT,INSERT ON OBJECT::dbo.eg100_samples TO eg100_writer;
GRANT CREATE TABLE TO eg100_tester;
GRANT ALTER,SELECT,INSERT,UPDATE,DELETE ON SCHEMA::dbo TO eg100_tester;
SELECT CAST(SERVERPROPERTY('ProductVersion') AS varchar(64)),CAST(SERVERPROPERTY('Edition') AS varchar(100));
SELECT name,type_desc FROM sys.database_principals WHERE name IN ('eg100_writer','eg100_tester');
GO
"@
    $sql | & docker exec -i -e SQLCMDPASSWORD $container /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -C -b -l 10 -h -1 -W
    if ($LASTEXITCODE -ne 0) { throw 'Provisioning incomplete; retain container and inspect.' }
    Write-Output ('Isolated SQL Server ready; pinned image: '+$image)
    Write-Output 'Gateway account: SELECT/INSERT on only dbo.eg100_samples; encrypted credentials retained locally.'
} finally {
    Remove-Item Env:MSSQL_SA_PASSWORD,Env:SQLCMDPASSWORD -ErrorAction SilentlyContinue
}
