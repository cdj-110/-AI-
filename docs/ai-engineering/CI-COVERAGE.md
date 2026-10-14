# CI 范围与接入说明
状态：可导入的手动首跑配置；本次未在 GitHub 执行，不宣称业务测试通过。

## 配置已适配的入口
- Web：两套生产构建。
- API：Prisma 生成、Nest 构建、功能权限回归；独立 job 运行设备范围测试（脚本自建 PostgreSQL 临时容器和测试迁移，结束销毁）。
- ingest：Prisma 生成、构建、启动恢复和点位权限同步模拟测试。
- Go 网关：先构建 webui 供 go:embed 使用，再默认 CGO 构建与 mapper/state 竞态测试。
- GoView：Linux build-cli，仅打包，不含类型检查。
使用各模块 npm package-lock；五个模块根依赖与锁文件声明已静态比对一致，但不等于 npm ci 已验证。

## 故意未覆盖的项目
浏览器设备分组脚本指定 msedge 且模块清单未声明 Playwright，需要单独固定依赖和浏览器版本。
Go 全套测试、真实工业设备、OPC DA 原生库与 IEC61850 MMS 标签构建未纳入首跑；先补各目标系统原生依赖再形成单独协议 job。
Python 网关、移动端、GoView 登录保存/预览、生产压力测试、OTA/真实授权均未验证。
只测 mapper/state 不代表完整 Go 网关质量门禁；不能据此直接发布。

## 初次执行前
确认仓库代码与这次本地快照对应，尤其未提交的新源码是否已经进入候选分支。只导入文件，不将工作区秘密、缓存和设备数据提交。
工作流使用 GitHub 托管 Ubuntu、Node 22、go.mod 指定 Go；没有生产连接。Node22 是首跑环境选择，不是已测兼容声明。GoView 内存上限 4GB 为起步设置，OOM 要调整 runner/内存并记录。
Actions 目前固定到具体 release tag，尚未固定完整提交 SHA；启用自动 PR 检查前应核实官方对应 SHA 并固定，后续由负责人审核升级。

## 官方依据
[工作流语法](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)
[AGENTS.md 加载规则](https://learn.chatgpt.com/docs/agent-configuration/agents-md)
[checkout v4.2.2](https://github.com/actions/checkout/releases/tag/v4.2.2)
[setup-node v4.4.0](https://github.com/actions/setup-node/releases/tag/v4.4.0)
[setup-go v5.5.0](https://github.com/actions/setup-go/releases/tag/v5.5.0)

