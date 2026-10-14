# 微控 Python 网关（首版）

这是现有 Go 网关的 Python 后端并行版本，直接复用 `apps/gateway-go/webui` 构建出的 Vue 页面和主要 HTTP/WebSocket 接口。默认端口为 `8089`，不会占用 Go 版的 `8088`。

## 首版已经迁移

- 原有 Vue 页面静态托管与登录会话；
- 配置读取、校验、原子保存和运行时重新加载；
- 全局唯一点位标识符校验；
- Modbus TCP、Modbus RTU 采集；
- 每个设备/通道独立固定周期调度，Modbus连接跨周期复用；
- Modbus 线圈和保持寄存器下发；
- OPC UA 点位读写和节点浏览；
- MQTT 多通道连接、QoS 1 上报和 TLS 客户端证书；
- 实时状态、点位状态和 WebSocket 增量推送；
- 手动采集、连接测试；
- 工程导入导出；
- 外部存储历史 JSONL 和 CSV 导出；
- 用户管理基础接口、网络状态占位接口和维护诊断接口。

## 尚未迁移

- IEC104 客户端自动兼容、总召、时钟同步及遥控；
- IEC104、Modbus TCP 转发服务；
- Siemens S7 采集和批量扫描；
- IEC61850 MMS 和 CID/ICD 解析；
- Go 版 CEL 边缘计算的完整语义；
- 4G/WiFi 参数写入、硬件/软件看门狗；
- 报文实时监控和云端远程配置闭环；
- 离线 MQTT 队列补传。

这些接口会继续保持与 Vue 前端一致，迁移完成前会明确返回 `501`，不会伪装为执行成功。

## Windows运行

```powershell
cd apps/gateway-python
.\install.ps1 -Python "Python解释器路径"
.\start.ps1
```

浏览器访问 `http://127.0.0.1:8089`，初始账号为 `admin`，密码为 `123456`。

也可以直接使用：

```powershell
.\.venv\Scripts\python.exe -m gateway.main --config config.local.json --listen 0.0.0.0:8089
```

## Linux/EG100运行

```sh
cd /opt/weikong/gateway-python
python3 -m venv .venv
.venv/bin/pip install -r requirements.txt
chmod +x start.sh
./start.sh config.local.json
```

当前 EG100 的 Python 版本和磁盘空间需要先确认。`asyncua` 等依赖明显大于 Go 单文件程序，因此正式部署时建议生成裁剪后的离线 wheel 包，并让 Go 与 Python 使用不同端口并行验收后再切换服务。

## 配置兼容

Python版直接读取 Go 版 JSON 配置的大部分设备和点位字段。建议先复制配置而不是移动原文件：

```powershell
Copy-Item ..\gateway-go\config.local.json .\config.local.json
```

Python版保存配置时采用临时文件加原子替换，避免掉电产生半截 JSON 文件。

## 测试

```powershell
.\.venv\Scripts\python.exe -m unittest discover -s tests -v
```
