import fs from 'node:fs'

const read=path=>fs.readFileSync(new URL(path,import.meta.url),'utf8')
const alarm=read('../src/components/AlarmView.vue')
const large=read('../src/components/LargeDevicePointWorkspace.vue')
const globalDialog=read('../src/components/GlobalConfirmDialog.vue')
const frame=read('../src/components/ConfirmationDialog.vue')

const checks=[
  ['告警规则删除使用统一确认服务',alarm.includes("title:'删除告警规则'")&&alarm.includes('confirmAction')&&!alarm.includes('window.confirm')],
  ['批量删除复用统一确认弹框',large.includes('<ConfirmationDialog')&&large.includes('title="批量删除点位"')],
  ['全局确认弹框复用统一框架',globalDialog.includes('<ConfirmationDialog')],
  ['统一弹框具备危险样式',frame.includes(':class="{ danger }"')&&frame.includes(':class="{danger,primary:!danger}"')],
  ['统一弹框支持 Esc 和遮罩取消',frame.includes("event.key!=='Escape'")&&frame.includes('@click.self="cancel"')],
  ['统一弹框具备无障碍语义',frame.includes('role="alertdialog"')&&frame.includes('aria-modal="true"')],
]

const failed=checks.filter(([,passed])=>!passed)
for(const [name,passed] of checks)console.log(`${passed?'PASS':'FAIL'} ${name}`)
if(failed.length)process.exit(1)
