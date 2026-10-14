# 组态开关接入点位下发（2026-10-13，已本地部署）

## 用户要求和候选范围

开关-01绑定点位后接入既有设备控制接口，不修改后端权限、网关能力或实际点位。

- SwitchFun配置增加允许下发（默认false，旧项目保持只读），NUMBER开/关数值（1/0默认）、STRING开/关文本（ON/OFF默认）；BOOLEAN下发原生true/false。值须不同、类型有效，文本长度限2048。
- 仅preview运行路径可控制。仅一个实时绑定点位，编辑、历史、多点位、未开启控制均不发送POST。允许选择数值点位，运行时仍要求网关可写能力；绑定区说明控制要求。
- 读取 `/devices/:id/commands/capabilities`，核对available及该identifier；每次控制前再检查，使用实际子设备ID。后端仅SUPER_ADMIN/TENANT_ADMIN、在线且云访问允许的网关子设备可写，未放宽权限。
- 人工确认设备点位及目标值后，一次只POST一次，UUID requestId供后端幂等；认证恢复不自动重放POST。仍使用现有平台认证请求通道，兼容平台code0与旧code200响应。
- 不乐观翻转开关、不把SENT当成功。轮询只读命令列表匹配id/requestId，显示SUCCEEDED/FAILED/TIMEOUT。实时上报继续决定开关显示，可识别配置的非默认数值/文本。
- 网络不确定/超时锁定重复控制，支持人工查询晚到回执；sessionStorage保存待确认身份（不存凭据），同一标签页组件重挂载/刷新保持保护。明确HTTP400/403/404拒绝可解除未创建指令的保护。无法确认或命令已离开最近30条列表时保持未知，不自动重发或宣称执行失败。
- 请求工具保留已知HTTP错误状态（不改变错误文案），用以区分明确拒绝与发送结果未知。

候选文件：SwitchFun/index.vue、config.ts、config.vue；RequestTargetConfig/index.vue；utils/switchControl.ts、request.ts；tools/verify-goview-switch-control.cjs。保留工作树其他改动，没有提交或部署。

## 验证

- tools/verify-goview-switch-control.cjs：实际Vue响应式/SFC编译+全部请求模拟，通过编辑/未开启/不可写/取消不写、布尔数值文本、单点实时、并发点击仅一次POST、不乐观翻转、成功/失败/明确拒绝/网络未知/超时持久保护/晚到回执。
- tools/verify-goview-single-data-model.cjs：通过；保持唯一数据模型选择修复。
- apps/api node scripts/verify-device-commands.cjs：后端角色/租户/网关能力、类型/离线、审计/幂等及回执桩检查通过，未真实操作设备或数据库。
- apps/goview-designer npm run build-cli：最终控制代码构建通过（存在既有eval、循环chunk和大包提醒）；不包含类型检查。其后仅补充绑定区提示文本并移除未用变量，需部署时再次构建。
- git diff --check：通过。
- 浏览器真实登录/绑定保存/发布预览控制、实机指令与回执、刷新多标签协调、现场通信失败演练：not_run。所有写入验证为模拟，不访问真实控制接口。

## 风险和回滚

默认不开启控制，运行界面必须由管理员明确启用；不同标签页的待确认保护不互通，不承诺全局互斥或设备实际动作验收。网关成功回执与实时值更新可能有延迟，界面明确区分。列表回执仅最近30条，未知时安全锁定。

回滚仅移除本次开关下发逻辑和配置，不修改既有下发后端、模型自动选择或其他编辑器修改；不删除指令审计记录、不执行设备控制或数据库回滚。

## 部署补充

已随自动/手动值映射在2026-10-13 15:55本地部署，详见goview-switch-auto-values-20261013.md。默认保持不允许下发；没有测试真实设备写入，不宣称实机控制验收。回滚镜像为weikong-switch-auto-rollback-goview:20261013-155349。
