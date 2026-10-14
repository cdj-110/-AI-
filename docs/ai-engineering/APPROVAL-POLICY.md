# 人工审批与执行边界 V1

2026-10-14 已通过 GitHub API 配置并回读：main 禁止强推和删除；要求一次 PR 审核、新提交使旧审核失效、解决讨论、分支与 main 保持最新，且 ci-required 必须由 GitHub Actions 应用（15368）报告成功；管理员同样受约束。

仓库仅有 cdj-110 一个用户，因此通过 GitHub Actions 创建机器人署名的草稿 PR，由 cdj-110 作为人工审查者。用户明确批准开启 GitHub 合并提供的“允许 Actions 创建和审批 PR”设置；本工作流只创建草稿 PR，没有审批或合并步骤。创建 PR 的 job 只有 contents:read、pull-requests:write；执行候选代码的 CI job 仅 contents:read，checkout 不保留凭据，不引用仓库 secrets，不使用 self-hosted runner。

本机 LangGraph 调用已登录的 Codex，生成候选并运行验证、AI 审查。模型不能代替最终人工审批。控制程序使用本人 GitHub 登录，仓库所有者仍具有管理权限；不声称该电脑完全没有主分支凭据或生产网络能力。

人工审核应核对最终 head SHA、CI 运行及未测范围。新增提交后旧审批失效，应重新检查。CI 失败、取消或跳过任一必需 job 时，汇总门禁失败；通过 CI 不自动合并。

本服务和工作流没有生产部署步骤。生产发布须另行明确授权，并绑定提交、制品摘要、环境、配置、迁移、备份和回滚计划。数据库与设备真实操作不由文档或 AI 自报结果放行。CI 不证明 OPC DA/MMS 实机能力或全系统安全性。
