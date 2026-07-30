# 网关产物目录

该目录统一存放 Go 与 Python 网关的发布包、历史构建、测试配置、日志和配置备份。源码仍保留在 `apps/gateway-go` 与 `apps/gateway-python`，避免构建、测试和部署脚本失效。

## 目录结构

```text
gateway-artifacts/
├─ go/
│  ├─ windows-amd64/
│  │  ├─ current/          # 当前 Windows x64 发布包和独立程序
│  │  └─ archive/          # 历史发布、根目录散落构建及旧部署包
│  ├─ arm64/
│  │  ├─ current/          # 当前通用 Linux ARM64 程序
│  │  └─ archive/          # 历史 ARM64 程序
│  ├─ EG100/
│  │  ├─ current/          # 当前 EG100 ARMv7 发布包和程序
│  │  ├─ archive/          # 历史 EG100 构建及旧部署包
│  │  ├─ backups/          # EG100 程序备份
│  │  └─ configs/          # EG100 部署、看门狗等历史配置
│  ├─ source-archives/     # 旧源码压缩包
│  ├─ test-data/           # 大规模点位等测试配置
│  ├─ logs/                # 本地构建和调试日志
│  └─ config-backups/      # Go 网关配置备份
└─ python/
   ├─ EG100/
   │  ├─ current/          # 当前 Python EG100 发布包
   │  └─ configs/          # 对应设备部署配置
   ├─ test-data/           # 1000/30000 点测试配置
   ├─ logs/                # Python 网关日志
   └─ config-backups/      # Python 网关配置备份
```

## 使用约定

- 部署或交付时优先从对应平台的 `current` 目录取文件。
- `archive` 仅用于追溯历史版本，不作为默认部署来源。
- `apps/*/deploy` 只保留部署脚本，不再混放编译产物。
- `config.local.json`、`.runtime` 和 Python `.venv` 仍位于各自源码目录，它们属于当前本机运行环境。
- 本目录中的二进制、压缩包、日志和测试数据默认不提交 Git；只提交本说明文件。
- 新生成的文件请按“语言 → 平台/设备 → current 或 archive”归档，替换 `current` 前先把旧文件移入 `archive`。
