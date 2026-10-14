import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { alarmPopupEnabled, alarmSoundEligible, detectNewActiveAlarms, sortActiveAlarms } from '../src/alarmNotifications.ts'
import { highestAlarmLevel } from '../src/alarmSound.ts'
import { alarmRuleTypeForDataType } from '../src/alarmRuleType.ts'

const event = (eventId, acknowledged = false) => ({
  eventId,
  ruleId: 'rule-1',
  ruleName: '温度过高',
  deviceKey: 'device-1',
  metric: 'temperature',
  level: 'CRITICAL',
  state: 'ACTIVE',
  acknowledged,
  message: '温度超过阈值',
  occurredAt: '2026-09-01T12:00:00+08:00',
  updatedAt: '2026-09-01T12:00:00+08:00',
})

const initial = detectNewActiveAlarms(new Set(), {
  active: [event('existing')], history: [], activeCount: 1, criticalCount: 1, unackedCount: 1,
}, false)
assert.equal(initial.events.length, 0, '首次加载不能把历史存量告警重复弹出')

const triggered = detectNewActiveAlarms(initial.eventIds, {
  active: [event('new'), event('existing')], history: [], activeCount: 2, criticalCount: 2, unackedCount: 2,
}, true)
assert.deepEqual(triggered.events.map(item => item.eventId), ['new'])

const acknowledged = detectNewActiveAlarms(new Set(), {
  active: [event('acknowledged', true)], history: [], activeCount: 1, criticalCount: 1, unackedCount: 0,
}, true)
assert.equal(acknowledged.events.length, 0, '已确认告警不能再次弹出')

const unsorted=[{...event('z'),ruleId:'b'},{...event('b'),ruleId:'a'},{...event('a'),ruleId:'a'},{...event('new'),occurredAt:'2026-09-01T12:00:01+08:00'}]
const originalIds=unsorted.map(e=>e.eventId)
assert.deepEqual(sortActiveAlarms(unsorted).map(e=>e.eventId),['new','a','b','z'])
assert.deepEqual(unsorted.map(e=>e.eventId),originalIds,'sorting must not mutate source')
assert.deepEqual(sortActiveAlarms([...unsorted].reverse().map(e=>({...e,acknowledged:true,value:999,updatedAt:new Date().toISOString()}))).map(e=>e.eventId),['new','a','b','z'],'live updates must not reorder tied alarms')

assert.ok(alarmPopupEnabled(event('legacy')), '旧事件兼容默认弹窗')
assert.ok(alarmPopupEnabled({...event('enabled'),popupEnabled:true}))
assert.equal(alarmPopupEnabled({...event('disabled'),popupEnabled:false}),false)
const hiddenEvent={...event('hidden'),popupEnabled:false}
const hidden=detectNewActiveAlarms(new Set(),{active:[hiddenEvent]},true)
assert.equal(hidden.events.length,1,'关闭面板提醒不能屏蔽告警检测和记录')
assert.equal(hidden.events.filter(alarmPopupEnabled).length,0)
assert.equal(alarmSoundEligible(hiddenEvent),false,'关闭面板提醒后默认不发声')
assert.equal(alarmSoundEligible({...hiddenEvent,soundWhenPopupDisabled:true}),true,'关闭面板提醒后可单独开启声音')
assert.equal(alarmSoundEligible(event('legacy')),true,'旧规则保持面板和声音提醒兼容')
assert.equal(detectNewActiveAlarms(hidden.eventIds,{active:[{...hiddenEvent,popupEnabled:true}]},true).events.length,0,'重新打开开关不补弹存量告警')

assert.equal(highestAlarmLevel(['WARNING', 'CRITICAL', 'INFO']), 'CRITICAL', '同一轮新告警只需播放最高等级提示音')
assert.equal(alarmRuleTypeForDataType('bool'), 'boolean', '布尔点位应自动使用状态告警')
for (const dataType of ['int8', 'uint64', 'float32', 'float64', 'currency', 'auto']) {
  assert.equal(alarmRuleTypeForDataType(dataType), 'threshold', `${dataType} 点位应自动使用数值阈值告警`)
}
for (const dataType of ['string', 'datetime', 'float64[]', 'string[]']) {
  assert.equal(alarmRuleTypeForDataType(dataType), 'quality', `${dataType} 点位应自动使用数据质量告警`)
}

const styles = await readFile(new URL('../src/styles.css', import.meta.url), 'utf8')
assert.ok(
  styles.includes('.legacy-device-workspace.is-running td.point-sheet-actions>button:first-of-type')
    && styles.includes('.legacy-device-workspace.is-running .mobile-point-card footer>button:first-of-type{display:none!important}'),
  '运行模式必须隐藏桌面表格和移动端卡片中的点位告警配置按钮',
)

const app = await readFile(new URL('../src/App.vue', import.meta.url), 'utf8')
assert.match(app, /function openAlarmCenter\(\)\{[^}]*alarmRuleTarget\.value=undefined;alarmCenterNonce\.value\+\+;applyNav\('alarms'\)\}/, '顶部告警入口必须清除点位规则上下文并进入告警管理')
assert.match(app, /const changed=await applyNav\(key\);if\(changed&&key==='alarms'\)\{alarmRuleTarget\.value=undefined;alarmCenterNonce\.value\+\+\}/, '侧栏告警入口必须在确认离开后清除点位规则上下文')
assert.match(app, /alarmPresentation\.update\(next.active,freshEvents\)/, '声音必须同时接收当前告警和新触发事件，支持持续播放与恢复停声')

const alarmView = await readFile(new URL('../src/components/AlarmView.vue', import.meta.url), 'utf8')
assert.match(alarmView, /<th>操作<\/th><th class="alarm-rule-enabled">启用<\/th>/, '规则启用列必须位于表格最后')
assert.match(alarmView, /role="switch"/, '规则启用控件必须使用可访问的滑动开关')
assert.match(alarmView, /:class="\{disabled:!rule\.enabled\}"/, '未启用规则必须整行置灰')
assert.match(alarmView, /class="alarm-rule-switch alarm-dialog-switch"/, '新增和编辑规则弹窗必须使用滑动开关')
assert.match(alarmView, /\{\{editing\.enabled\?'已启用':'已关闭'\}\}/, '弹窗开关必须显示明确的启用状态')
assert.doesNotMatch(alarmView, /<select v-model="editing\.type"/, '告警类型不能再由用户手动选择')
assert.match(alarmView, /根据 \{\{dataTypeLabel\(selectedDataType\(editing\)\)\}\} 自动判断/, '弹窗必须用中文类型说明自动判断依据')
assert.match(alarmView, /\/api\/alarms\?activeOnly=1/, '活动告警轮询不能携带全部历史记录')
assert.match(alarmView, /\/api\/alarms\/history\?\$\{params\}/, '告警历史必须使用服务端分页接口')
assert.match(alarmView, /historyPage\.totalPages/, '告警历史必须展示并控制分页状态')
assert.match(alarmView, /historyFilters\.level/, '告警历史必须支持等级筛选')
assert.match(alarmView, /\.alarm-table-wrap\{max-height:none;overflow-x:auto;overflow-y:visible\}/, '告警表格不能产生第二个纵向滚动条')

console.log('alarm notification verification passed')
