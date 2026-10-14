$ErrorActionPreference='Stop'
$name='eg100-mysql-integration'
$old='eg100-mysql-integration-local'
$info=(& docker inspect $name | ConvertFrom-Json)[0]
if ($LASTEXITCODE -ne 0 -or $info.Config.Labels.purpose -ne $name) { throw 'Unexpected container.' }
if (!($info.Mounts | Where-Object { $_.Name -eq 'eg100-mysql-integration-data' -and $_.Destination -eq '/var/lib/mysql' })) { throw 'Unexpected data volume.' }
if ((& docker ps -a --format '{{.Names}}') -contains $old) { throw 'Retained local container already exists.' }
& docker stop -t 30 $name
if ($LASTEXITCODE -ne 0) { throw 'Stop failed.' }
& docker rename $name $old
if ($LASTEXITCODE -ne 0) { & docker start $name; throw 'Rename failed.' }
& docker run -d --name $name --label purpose=eg100-mysql-integration --memory 768m --cpus 1 -p 127.0.0.1:3306:3306 -p 192.168.1.141:3306:3306 -v eg100-mysql-integration-data:/var/lib/mysql mysql@sha256:85b9bf2e29cf836ecb8c2a15a935d4ba0c606631dff1dd79531a11983c638f2a --character-set-server=utf8mb4 --collation-server=utf8mb4_bin --max-connections=40 --innodb-buffer-pool-size=134217728
if ($LASTEXITCODE -ne 0) { throw 'LAN container did not start; old stopped container and database volume retained for recovery.' }
Write-Output 'LAN binding added with explicit user approval. Existing data volume retained; old localhost container remains stopped.'
