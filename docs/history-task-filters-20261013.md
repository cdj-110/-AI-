# 历史任务记录筛选（2026-10-13，已本地部署）

## 行为和候选

用户要求历史表格支持筛选。HistoryTaskEditor查看区域增加设备、点位、关键词、数值上下限及时间范围，点击筛选生效；重置恢复整个任务条件。点位随设备联动，数值范围必须先指定点位（上下限包含边界）；临时筛选不修改任务，翻页和详情导出使用已应用条件。列表导出继续导出整个任务，没有隐式继承弹窗筛选。

API HistoryTaskFilterDto白名单校验字段/长度，任务服务校验设备和点位属于已保存任务、时间在原任务内、数值有限且下限不大于上限。原任务所有设备权限仍先复验，不能用筛选隐藏已撤销权限后绕过检查。任务配置、原始遥测不变；没有迁移。

DevicesService参数化JSON请求点位附加数值范围，数据库WHERE筛选原始数值，文本和null不强制转换；同一WHERE用于COUNT与分页，CSV复用同一查询。旧调用可不提供筛选，保持原有分页参数位置，图表默认聚合无变化。保留20点位/31天/2000条CSV和套餐保留期限。

候选文件：history-data-tasks.service.ts、history-data-tasks.controller.ts、devices.service.ts、HistoryTaskEditor.vue，以及任务/API/浏览器验证脚本、新独立数据库验证脚本。保留其他工作树修改，未提交。

## 验证与边界

- apps/api npm run build：通过。
- verify-history-data-tasks.cjs：查询/导出传递同一筛选、0值边界、未知设备/点位拒绝、无点位数值筛选拒绝、Infinity/逆序/越界时间拒绝、不写任务，通过。
- verify-history-retention.cjs、verify-tenant-user-action-permissions.cjs：通过。
- verify-history-filters-db.cjs：一次性独立PostgreSQL16、合成遥测，实际SQL验证包含边界数值筛选、0值、文本/null排除、分页/总数、关键词组合、CSV同条件、无筛选兼容、不读取未选设备，通过。测试容器已移除，不访问现有业务库。
- apps/web npm run build:all：管理/租户类型检查与构建通过。
- verify-history-tasks-ui.cjs：隔离Chrome全部API模拟，设备归属、弹窗、关键词/点位/数值筛选、翻页保留条件、CSV同条件、重置、编辑、关闭/重开、移动布局，通过。
- 真实登录HTTP闭环、部署后检查、大数据性能、图表聚合专项SQL回归：not_run。未部署，不把模拟结果视为生产验收。

## 风险/回滚

前后端应一起更新以使服务端筛选生效；条件可缩小不可扩展任务范围。回滚仅恢复本次筛选参数和UI，保留之前的点位归属/弹窗修改，不执行数据库回滚或历史删除。

## 本地部署结果

2026-10-13 14:57（Asia/Shanghai），按用户“重新部署”要求更新API和tenant web。镜像构建/启动成功；仅重建这两项服务，没有迁移、历史数据写入/删除或网关重启。

回滚镜像保留：`weikong-history-filter-rollback-api:20261013-145606`、`weikong-history-filter-rollback-web:20261013-145606`。需要回滚时恢复两项服务镜像，不回滚数据库。

已部署后端产物任务筛选校验桩检查通过；已部署前端隔离浏览器检查通过（关键词/点位/数值筛选、翻页保留条件、CSV同条件、重置和弹窗编辑等，全部API模拟）。数据中心HTTP200，服务运行正常、数据库健康，近期API日志未发现匹配的ERROR/Unhandled/Prisma错误。历史清理仍为false。真实用户HTTP登录保存/筛选/导出和大数据性能仍为not_run。
