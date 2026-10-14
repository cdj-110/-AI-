# Ubuntu ARM64 测试部署

该目录用于在 Ubuntu ARM64 测试机上运行 Go 网关和原生 OPC DA 驱动。
生产 EG100 仍使用 `deploy/eg100` 下的 ARMv7 流程。

运行结构保持为单个 `weikong-gateway` 主进程，由 systemd 负责拉起：

- `GOMEMLIMIT=64MiB`、`GOGC=70`；
- `MemoryHigh=64M`、`MemoryMax=80M`，用于提前暴露 EG100 级别的内存问题；
- `TasksMax=64`、`LimitNOFILE=4096`；
- 日志写入 `/var/log/weikong-gateway/gateway.log`，由 logrotate 按 2MB、5 份轮转；
- 原生库位于 `/opt/weikong-gateway/lib`，配置文件权限为 `0600`。

部署脚本从环境变量读取凭据，不会把密码写入仓库：

正式二进制必须使用 Ubuntu 20.04 / glibc 2.31 兼容工具链，避免在测试机上
出现 `GLIBC_2.32 not found`。可复现构建镜像位于 `Dockerfile.glibc231`。

```powershell
docker build -f deploy/ubuntu-arm64/Dockerfile.glibc231 -t wk-ubuntu-arm64-glibc231 .
docker run --rm -v "${PWD}:/src" wk-ubuntu-arm64-glibc231 bash -lc `
  "GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc-9 CGO_LDFLAGS='-L/src/build/ubuntu-arm64/lib -Wl,-rpath,/opt/weikong-gateway/lib' go build -buildvcs=false -tags opcda -trimpath -ldflags='-s -w' -o /src/build/ubuntu-arm64/weikong-gateway ./cmd/gateway"
```

```powershell
$env:WK_REMOTE_PASSWORD = 'CHANGE_ME_LOCAL_ONLY'
$env:WK_OPCDA_PASSWORD = 'CHANGE_ME_LOCAL_ONLY'
python deploy/ubuntu-arm64/deploy_ubuntu_arm64.py deploy `
  --gateway build/ubuntu-arm64/weikong-gateway `
  --library build/ubuntu-arm64/lib/libwk_opcda.so.0.1.0 `
  --service deploy/ubuntu-arm64/weikong-gateway.service `
  --logrotate deploy/ubuntu-arm64/weikong-gateway.logrotate
```

可复用的验证命令包括 `inspect`、`status`、`http-test`、`mode-test` 和
`restart-test`。`mode-test` 会把测试设备热切换到异步订阅，并验证 PID 不变、
回调质量、源时间、接收时间和陈旧标记。
