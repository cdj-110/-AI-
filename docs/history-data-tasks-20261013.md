# 历史数据任务：数据中心流程改版（2026-10-13，已本地部署）

## 用户要求与交付

按用户最新要求，数据中心不再把所有设备/点位/时间配置与结果堆在一个页面。

- `/data-center` 只展示任务列表，包含名称、设备/点位、时间范围、创建人和操作；支持搜索、分页、查看、导出、删除。
- 新增任务通过弹框，填写名称、设备、点位、固定历史时间范围后保存到服务器。
- `/data-center/:id` 独立详情页，默认只展示保存条件摘要、历史结果、分页及导出；点击编辑才进入配置，编辑时隐藏结果区域，取消恢复原配置。
- 名称/选择点位/时间均可修改；刷新页面仍读取服务器保存的配置。删除任务须确认，仅删除配置，不删除遥测。
- 任务是“历史查询任务”，不是后台永久数据归档或冻结快照；查看/导出时重新读取原始历史记录，仍受套餐期限限制。超出保存范围时提示修改任务条件，不擅自重写保存范围。异步全量大文件任务不在本次范围，CSV仍最多2000条并明确提示截断。

## 持久化与安全

- 新增业务库 `HistoryDataTask` 模型，tenant外键、创建者、名称、JSON点位、设备名称快照、时间、创建/修改时间。迁移 `20261013140000_history_data_tasks` 仅建表/索引，不复制或清理遥测。
- API `/api/history-data-tasks` 的列表/新增及 `/:id` 查看/修改/删除、`/:id/records` 分页、`/:id/export` 导出。
- TENANT_USER只访问自己创建的本租户任务；TENANT_ADMIN访问本租户任务；统一需要DEVICE_VIEW，不放宽为未登录/跨租户访问。
- 任务详情、查询、导出重新检查设备访问范围；失去设备权限时列表不可查看/导出。任务创建及修改复用历史服务验证保存期限、点位有效性和单次跨度；每次查询/导出继续验证，不只相信任务保存时的权限。
- 创建配置校验必填名称、ISO开始/结束时间、嵌套UUID/标识符，最多20点位，不允许重复点位。创建请求不能覆盖tenantId/createdById。
- 老数据中心查询/导出端点保留兼容，但前端工作流已迁移到任务。

## 候选范围与验证

候选：API Prisma模型/追加迁移、HistoryDataTasksService/Controller、DevicesModule注册；web DataCenter列表、HistoryTaskEditor配置/结果组件、HistoryTaskDetail独立页、路由及菜单高亮。保留原工作树，未创建提交。

- API `npm run prisma:generate; npm run build`：通过。
- `node scripts/verify-history-data-tasks.cjs`：必填DTO、保存定义、成员/租户隔离、管理员范围、权限撤销、按存储条件查询导出、修改删除，通过；全为桩，没有业务库写入。
- `node scripts/verify-history-retention.cjs`、`node scripts/verify-tenant-user-action-permissions.cjs`：通过。
- `node scripts/verify-history-tasks-db.cjs`：一次性独立PostgreSQL实际执行新表迁移，Prisma创建/读取JSON和时间、更新、所有者筛选、删除配置不影响遥测，通过。临时容器已移除，没有访问现有业务库。
- Web `npm run build:all`：两套类型检查/构建通过。
- Web `node scripts/verify-history-tasks-ui.cjs <bundled-playwright-path>`：隔离Chrome/所有API模拟，任务列表无配置表单、新增弹框、两设备启用点位、保存后独立路由、刷新配置、编辑、CSV下载、返回列表删除、手机无整页横向溢出、无页面异常，通过。
- 截图 `outputs/history-data-tasks-20261013/` 为模拟数据。
- `git diff --check`：通过。
- 实现阶段未部署；随后按用户要求完成下述本地部署。历史清理未启用，未改变网关。
- 真实用户HTTP登录闭环、多成员并发修改、长期任务数量/大数据性能：not_run。不用模拟测试冒充正式验收。

## 下次部署与回滚

用户明确请求部署后，先备份本地业务库，核对唯一待执行迁移，再应用建表迁移并更新API/tenant web。不要只更新前端而漏掉新任务表。

回滚到上一版API/tenant web即可恢复原数据中心界面；保留新增任务表和用户任务，不破坏性删表、不删除历史数据。独立权限/大批量异步导出可以后续按需求追加。

## 本地部署结果（2026-10-13 13:51，Asia/Shanghai）

- 用户请求“重新部署”，更新本地 API 与 tenant web；管理端、GoView、网关及其他服务未重启。
- 业务库备份：`C:\Users\18507\Documents\物联网平台\deployment-backups\history-data-tasks-20261013-134544\business-before.dump`，235663字节，备份目录结构校验297条通过。
- 回滚镜像：`weikong-history-tasks-rollback-api:20261013-134544`、`weikong-history-tasks-rollback-web:20261013-134544`。回滚应使用这两份镜像恢复服务，保留任务表和记录，不执行删表。
- API/web镜像构建通过；核对唯一待执行迁移为 `20261013140000_history_data_tasks`，正式执行成功，仅创建任务表及索引。部署后 Prisma 读取任务表成功（检查时0条），没有测试性业务库写入。
- 服务运行正常，数据库健康；`/data-center` 返回200，新任务列表与记录接口未登录返回401，未放宽鉴权。
- 已部署后端产物任务桩检查通过：必填条件、租户/成员隔离、管理员范围、权限撤销、固定条件查询/导出、修改/删除不写遥测。近期API日志未发现匹配的ERROR/Unhandled/Prisma错误。
- 已部署前端浏览器检查通过：列表、新建弹框、独立详情路由、刷新、编辑、CSV、删除配置、手机布局、无页面异常；全部API模拟，没有实际登录或创建真实任务。截图：`outputs/history-data-tasks-20261013-deployed/`。
- `HISTORY_CLEANUP_ENABLED=false`，未执行历史删除。真实登录HTTP闭环和大数据性能仍为not_run。
