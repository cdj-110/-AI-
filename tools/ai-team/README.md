# 微控 AI 研发团队 V1：本机 Codex 调度

LangGraph 顺序执行规划、开发、验证、AI 审查；每个任务在独立 Git 克隆中生成候选。Codex CLI 使用现有 ChatGPT 登录，不需要 OpenAI API 密钥。不同角色是顺序调用的职责划分，不代表独立人工审批。

## 初次配置

需要 Python 3.12、Node 22、Git、GitHub CLI、Docker Desktop 和当前 Codex CLI。运行 `codex login`、`gh auth login` 完成本人登录。不要把登录令牌、用户环境文件或任务日志提交到仓库。

在本目录创建虚拟环境，安装 `requirements.txt`（或固定完整依赖的 `requirements.lock.txt`），将 `config.example.json` 复制为 `config.local.json`，填写隔离仓库、私有状态目录、Codex 可执行文件、GitHub 仓库。

隔离基线必须是已人工合并的最新 main。每次接收任务前，在干净隔离仓库中 `git fetch origin main` 并切换到 `origin/main`；不要在原项目工作树执行初始化或依赖安装。

## 提交任务

从本目录运行，python 应为所建虚拟环境的解释器：

```powershell
python team.py doctor
python team.py run --task pilot-task.json
python team.py publish --job 任务输出的ID
python team.py serve
```

run 生成已验证、经 AI 审查的候选；publish 再次校验提交与文件摘要，确认基于当前远端 main 后推送 ai/task-*。默认不自动推送。失败任务、固定输出测试、被修改候选或过期基线均不能发布。

GitHub push 触发 Linux CI；全部必需检查成功后，机器人创建草稿 PR。cdj-110 检查差异与 CI，标记 Ready for review、提交 Approve 并自行决定是否合并。服务不执行审批、合并、部署、数据库迁移或设备控制。

## 模块与验证

- docs：仅 docs/ai-tasks/ 下的 Markdown；检查路径、文档内容和凭据模式，不宣称运行应用测试。
- web、api、ingest：仅对应 src/ 下的文件。运行前必须配置经摘要固定的 Linux 依赖镜像。清单不匹配、测试失败或镜像未配置均停止任务。
- Go 网关、GoView、数据库 schema、CI/审批策略、授权签发及商业规则变更不在本版模型可写范围内；需要单独明确范围与验证方案。

在隔离仓库根目录执行 `docker build -f tools/ai-team/Dockerfile.checks -t weikong-ai-checks:local .`，用 `docker image inspect weikong-ai-checks:local --format '{{.Id}}'` 获取内容 ID，将同一 sha256:... 分别填入 test_images.web/api/ingest。不要填可变 tag。镜像内通过 npm ci 安装依赖；任务容器断网、限制内存与 CPU、不挂载 Docker socket、凭据或生产目录，只读挂载任务克隆后复制到临时工作目录运行测试。清单变化需重建镜像。

最终 GitHub CI 使用 Node 22 和 Ubuntu，执行真实前端类型检查、两套构建和故意类型错误的失败验证，API/ingest 指定回归、一次性 PostgreSQL 设备范围集成测试、GoView 打包、Go race 与默认网关构建。GoView 打包不等于其类型检查；不覆盖浏览器全流程或 OPC DA/MMS 硬件。

## 状态与恢复

面板默认 http://127.0.0.1:18741，仅本机只读展示。任务状态、命令日志及候选保存在 state_dir；日志可能含源码，不随公开源码上传。服务退出不删除记录；V1 不自动续跑中断任务，核对后以新任务重新运行。

本机 Codex 使用本人 GitHub/ChatGPT 登录及权限，控制程序属于可信操作环境；路径白名单、只读模型沙箱、容器和分支保护不应被描述为完全隔离的多用户安全边界。主分支管理员权限依旧由仓库所有者持有。

数据库事实：云平台 PostgreSQL 业务库、TimescaleDB 时序库；Go 网关 MySQL 是数据输出能力。生产版本、部署提交和镜像摘要另行核实。
