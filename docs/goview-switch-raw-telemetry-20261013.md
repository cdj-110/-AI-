# 开关布尔实时值类型修复（2026-10-13，已本地部署）

## 原因与只读取证

用户反馈自动和手动均不能下发。之前只读检查：测试项目开关controlEnabled=true，实时绑定开关设备mbtcp_P01；管理员capabilities显示在线可写且类型BOOLEAN，最新历史原始值false。platformTelemetry.toValue将false通过Number转换为0，造成开关严格匹配BOOLEAN时状态未知，控制被禁用。没有实际调用控制POST。

## 候选修改

- platformTelemetry.updateComponents对SwitchFun使用原始点位值，不数字化布尔/文本；即使上一次值null，也能恢复接收有效值。其他组件既有数字转换保持不变。
- PreviewRenderList/test.ts的兼容实时分支对SwitchFun同样保留原始类型；没有匹配自己的点位时不复制全局listCount或其他组件值。
- SwitchFun/index.vue明确提示手动配置类型与实际点位类型不一致，并禁用控制，不能通过改变UI类型绕过后端实际类型。BOOLEAN点位手动模式仍须选择布尔，并配置不同布尔开/关值。

保留默认不允许下发、编辑不写、管理员/网关权限、人工确认和未知回执保护。没有修改API、点位配置、历史数据、订阅或真实设备状态。

## 验证

- tools/verify-goview-switch-control.cjs：直接执行实际platformTelemetry更新函数及兼容更新函数，验证false/true、文本1/0、ON/OFF、null和后续恢复、未匹配设备不串值、其他Number组件数字转换不变；实际Vue开关自动/手动BOOLEAN从保存样例0收到false后可生成一次原生布尔控制请求（全模拟），通过。
- 既有取消/只读/编辑/权限、值映射、发送一次、成功失败未知和持久保护回归通过；唯一数据模型回归通过。
- npm run build-cli构建通过（既有eval/循环chunk/大包警告），不包含类型检查。
- git diff --check通过。真实浏览器登录/实机下发验收、部署后验证not_run。未部署，所有写入测试为模拟。

## 回滚

只撤销本次SwitchFun原始类型处理及手动类型提示，不覆盖其他工作树功能，不修改或删除数据。回滚会重新引入布尔识别问题，不推荐正常运行时回退此修复。

## 本地部署结果

2026-10-13 16:26（Asia/Shanghai），按用户要求构建并更新goview-designer，仅重启组态容器，API/数据库/网关等未重启，没有迁移或实际设备指令。

回滚镜像 `weikong-switch-raw-rollback-goview:20261013-162318`。构建/启动成功；首页及JS/CSS为HTTP200，近期组态日志无匹配error/emerg。主入口index-d6bb709b.js与开关index-fb5a93ef.js的部署SHA256与已验证本地最终构建一致。

自动/手动布尔识别、两条实时原始类型路径、下发与回执保护的源码模拟检查再次通过。初次静态搜索valueTypeMismatch未命中，因为局部变量名被压缩，改用产物文件校验确认，不将搜索结果当运行故障。真实浏览器登录/实际设备指令及回执仍为not_run。此部署不修改原任务或开关配置，手动模式仍需选择实际BOOLEAN类型。
