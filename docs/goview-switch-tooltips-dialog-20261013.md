# 开关说明悬浮提示和统一确认弹框（2026-10-13，已本地部署）

用户要求：常驻说明放在配置项后方，悬停查看；确认弹框采用统一样式。

候选：SwitchFun/config.vue移除两个常驻n-alert，在允许下发和值识别方式控件后添加HelpOutlineIcon按钮+n-tooltip（与编辑器现有Naive UI提示一致，最大280px，左侧展开，按钮有aria-label与键盘焦点样式）。SwitchFun/index.vue移除window.confirm，采用现有goDialog统一主题、信息图标及按钮，标题确认点位下发，包含点位/目标值和设备写入提醒，按钮确认下发/取消。

控制仍等待明确确认才POST，关闭/X/离场视为取消；确认后复查组件存在/运行路径/绑定代次，取消无指令。未修改下发权限、默认关闭控制、回执保护或数据类型转换。

验证：tools/verify-goview-switch-control.cjs两提示/无常驻alert/无原生confirm、实际SFC及配置模板编译、确认下发单次POST/取消/X/离场零POST、其余控制及两条实时路径回归通过；唯一模型回归通过。npm run build-cli通过，不含类型检查，仍有既有大包/循环chunk/eval提醒。git diff --check通过。真实登录浏览器悬停/主题视觉与实机控制not_run。

未部署、未发送真实设备指令、未修改数据库。回滚仅恢复本次提示/确认UI，不覆盖其他工作树修改；无需数据库回滚。

## 本地部署结果

2026-10-13 16:46（Asia/Shanghai）按用户要求构建并更新goview-designer；仅重启组态服务，未重启API/数据库/网关、未迁移或操作设备。

回滚镜像：`weikong-switch-ui-rollback-goview:20261013-164409`。构建与启动成功，首页/主JS/CSS为HTTP200，入口index-048774e4.js及开关相关组件/配置包校验与本地最终验证构建一致。服务正常，近期组态日志无匹配error/emerg。

源码模拟确认/取消/X/离场及开关数据类型/回执保护回归再次通过；真实登录浏览器主题悬浮视觉和设备指令仍为not_run。没有使用真实下发测试验收UI。
