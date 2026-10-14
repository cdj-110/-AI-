import fs from 'node:fs'

const storage=fs.readFileSync(new URL('../src/components/StorageView.vue',import.meta.url),'utf8')
const checks=[
  ['历史点位在任务弹框中选择',storage.includes('class="history-task-modal"')&&storage.includes('选择历史点位')],
  ['提供新增和编辑历史任务',storage.includes('openCreate')&&storage.includes('openEdit')&&storage.includes('commitTask')],
  ['提供全部/指定点位保存范围',storage.includes('<option value="all">全部点位</option>')&&storage.includes('<option value="selected">指定点位</option>')],
  ['使用分页点位接口',storage.includes("'/api/history-storage/points?'")&&storage.includes('changePointPage')],
  ['支持搜索与多选',storage.includes('pointSearch')&&storage.includes('togglePoint')&&storage.includes('选择当前页')],
  ['按任务保存点位白名单',storage.includes('tasks:HistoryTask[]')&&storage.includes('JSON.stringify(historyForm)')],
  ['空白名单阻止创建任务',storage.includes('请至少选择一个历史点位')],
  ['支持周期与变化保存',storage.includes('<option value="interval">按周期保存</option>')&&storage.includes('<option value="change">变化时保存</option>')],
  ['变化保存支持数值死区',storage.includes('changeDeadband')&&storage.includes('数值变化死区')],
  ['展示 SQLite 与容量淘汰状态',storage.includes('SQLite 数据库')&&storage.includes('达到上限自动淘汰最旧数据')],
  ['支持开始和截止时间',storage.includes('开始时间')&&storage.includes('截止时间')&&storage.includes('datetime-local')],
  ['支持永久和持续时长',storage.includes('永久运行')&&storage.includes('持续时长')],
  ['展示任务生命周期状态',storage.includes('运行中')&&storage.includes('待开始')&&storage.includes('已完成')],
  ['默认使用本地SQLite并允许迁移外部存储',storage.includes("const localHistoryPath='/userdata/weikong/data'")&&storage.includes('本地存储（默认）')&&storage.includes('迁移数据库')],
  ['展示运行时磁盘保护与数据库恢复状态',storage.includes('历史写入已暂停')&&storage.includes('recoveryCount')&&storage.includes('释放空间后历史写入会自动恢复')],
  ['支持历史分页查询与趋势图',storage.includes("'/api/history-storage/query?'")&&storage.includes('historyChart')&&storage.includes('changeHistoryPage')],
  ['支持7/30/90天保存期限',storage.includes('保留 7 天')&&storage.includes('保留 30 天')&&storage.includes('保留 90 天')],
  ['展示任务保存条数空间和最后写入',storage.includes('taskStats')&&storage.includes('estimatedBytes')&&storage.includes('lastWriteAt')],
  ['截止任务自动进入完成归档区',storage.includes('completedTasks')&&storage.includes('已完成任务')&&storage.includes('自动归档')],
  ['存储迁移需要确认并调用迁移接口',storage.includes('migrateHistory')&&storage.includes("'/api/history-storage/migrate'")&&storage.includes('原数据库保留为备份')],
  ['支持按任务点位和时间后台导出',storage.includes('createHistoryExport')&&storage.includes("'/api/history-storage/exports'")&&storage.includes('后台导出当前条件')],
  ['支持定时备份轮换与可选加密',storage.includes('backupIntervalHours')&&storage.includes('backupKeep')&&storage.includes('encryptBackups')&&storage.includes('AES-GCM')],
  ['提供数据库校验备份恢复和维护入口',storage.includes('数据库维护')&&storage.includes("runMaintenance('check')")&&storage.includes("runMaintenance('backup')")&&storage.includes("runMaintenance('restore'")],
  ['支持小时天聚合与自适应趋势标识',storage.includes('aggregationEnabled')&&storage.includes('小时聚合')&&storage.includes('天聚合')&&storage.includes("resolution:'auto'")],
  ['备份支持校验值下载和删除',storage.includes('sha256')&&storage.includes('downloadBackup')&&storage.includes("runMaintenance('delete_backup'")],
  ['历史功能按标签页拆分且任务管理默认优先',storage.includes("const activeHistoryTab=ref<HistoryTab>('tasks')")&&storage.includes("selectHistoryTab('query')")&&storage.includes("selectHistoryTab('exports')")&&storage.includes("selectHistoryTab('settings')")&&storage.includes("selectHistoryTab('maintenance')")],
  ['导出和维护数据进入标签时按需加载',storage.includes("if(tab==='exports')void loadExportJobs()")&&storage.includes("if(tab==='maintenance')void loadMaintenance()")],
  ['趋势图支持悬浮点位值单位和毫秒时间戳',storage.includes('updateHistoryChartHover')&&storage.includes('history-chart-tooltip-value')&&storage.includes("selectedQueryPoint?.unit")&&storage.includes('formatHistoryTime(hoveredHistoryPoint.at)')],
  ['趋势图提供面积、十字线和聚焦点视觉反馈',storage.includes('history-chart-area')&&storage.includes('history-chart-focus')&&storage.includes('history-chart-line-shadow')],
]
const failed=checks.filter(([,passed])=>!passed)
for(const [name,passed] of checks)console.log(`${passed?'PASS':'FAIL'} ${name}`)
if(failed.length)process.exit(1)
