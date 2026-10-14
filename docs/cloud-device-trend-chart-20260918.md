# 云平台设备趋势图空白修复

## 原因

设备详情默认展示概览，实时数据面板使用 `v-show` 隐藏。旧代码在请求趋势数据后直接初始化 ECharts，隐藏容器尺寸为零；进入实时数据标签时未触发图表尺寸更新，因此统计值正常但图形空白。

## 调整

- 仅在实时数据面板可见、容器尺寸有效时初始化和绘制。
- 进入实时数据标签后重绘，保留已经读取的趋势数据。
- 使用 ResizeObserver 跟踪容器变化，覆盖标签切换及不触发窗口 resize 的布局变化。
- 点位为空或切换设备造成容器重建时，销毁旧实例并绑定新容器。
- 页面退出时释放实例和观察器，阻止未完成请求在已退出页面创建图表。

## 验证

`apps/web/scripts/verify-device-trend.mjs` 使用实际 DeviceDetail、Element Plus 和 ECharts，以及本地模拟只读接口，检查首次进入、反复切换标签、容器宽度变化、时间范围切换、无点位设备及容器重建。浏览器无页面错误和 ECharts 零尺寸警告。

云平台类型检查和租户版生产构建通过。截图：`apps/web/artifacts/device-trend-20260918/trend-chart.png`。

## 部署

2026-09-18 已更新云平台租户前端 `web`（5174）。接口、管理平台、数据库、Redis、MQTT 及遥测接收服务的容器均保持不变，无数据库迁移。

旧版本备份：`artifacts/cloud-trend-deploy-20260918024830/web.original.tar.gz`；回退镜像：`wk-cloud-trend-rollback:20260918024830`。

实际设备数据验证通过：首次进入实时数据、反复切换标签、窗口尺寸变化、切换时间范围均正常绘制，画布存在可见图形，接口无失败、页面无错误。验证未提交配置或数据变更。记录及图表截图位于 `artifacts/cloud-trend-verification-20260918`。

ECharts 官方尺寸及容器生命周期说明：https://echarts.apache.org/handbook/en/concepts/chart-size/
