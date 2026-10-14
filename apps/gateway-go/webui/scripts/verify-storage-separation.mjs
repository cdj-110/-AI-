import fs from 'node:fs'

const storage=fs.readFileSync(new URL('../src/components/StorageView.vue',import.meta.url),'utf8')
const cloud=fs.readFileSync(new URL('../src/components/CloudView.vue',import.meta.url),'utf8')
const app=fs.readFileSync(new URL('../src/App.vue',import.meta.url),'utf8')

const checks=[
  ['历史页不再承载断线续存接口',!storage.includes("'/api/storage'")&&!storage.includes('saveOffline')],
  ['历史页只显示历史数据入口',storage.includes('<h2>历史数据</h2>')&&!storage.includes('<h2>断线续存</h2>')],
  ['历史保存使用统一滑动开关',storage.includes('class="edge-master-switch"')&&storage.includes("historyForm.enabled?'已启用':'已关闭'")],
  ['历史保存关闭时折叠任务详情',storage.includes('<template v-if="historyForm.enabled">')],
  ['云平台页承载断线续存',cloud.includes('<h3>断线续存</h3>')&&cloud.includes("api<OfflineCacheStatus>('/api/storage')")],
  ['断线续存使用统一滑动开关',cloud.includes('class="edge-master-switch"')&&cloud.includes("offlineForm.enabled?'已启用':'已关闭'")],
  ['断线续存随云配置保存',cloud.includes("api('/api/storage',{method:'PUT'")&&cloud.includes('JSON.stringify(offlineForm)')],
  ['导航已区分历史和云端续存',app.includes("label:'历史数据'")&&app.includes("description:'MQTT、微控云与断线续存'")],
]

const failed=checks.filter(([,passed])=>!passed)
for(const [name,passed] of checks)console.log(`${passed?'PASS':'FAIL'} ${name}`)
if(failed.length)process.exit(1)
