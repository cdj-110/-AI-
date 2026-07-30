<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { api } from '../api'
import { useConfigStore } from '../stores/config'

type Provider = { baseUrl:string; apiKey:string; model:string; workspaceId?:string }
type Settings = { enabled:boolean; deepseek:Provider; bailian:Provider }
type Capabilities = { chat:boolean; pdf:boolean }
type GatewayAction = {
  id:string;tool:string;title:string;summary:string;arguments:Record<string,any>
  steps?:string[];state?:'pending'|'applying'|'applied'|'failed'|'cancelled';error?:string;selectedKeys?:string[]
}
type GatewayQuestion = {
  question:string;reason?:string;options:string[];allowCustom:boolean
  selected?:string;custom?:string;answered?:boolean;error?:string
}
type GatewayFormField = { key:string;label:string;type:'text'|'number'|'select';required:boolean;value:string;placeholder?:string;options?:string[] }
type GatewayForm = { title:string;reason?:string;fields:GatewayFormField[];submitted?:boolean;error?:string }
type ChatMessage = { role:'user'|'assistant'; content:string; action?:GatewayAction; question?:GatewayQuestion; form?:GatewayForm; hidden?:boolean }
type Draft = {
  id:string; baseRevision:string; channelKey:string; targetMode:string; targetDeviceKey?:string
  device:any; warnings?:string[]; blockingIssues?:string[]
}
type Job = {
  id:string; status:string; stage:string; progress:number; message?:string; error?:string; draft?:Draft
}
type Session = { permissions?:string[] }

const store=useConfigStore()
const tab=ref<'chat'|'pdf'>('chat')
const settingsOpen=ref(false), settingsSaving=ref(false), settingsError=ref(''), lastError=ref('')
const settings=reactive<Settings>({
  enabled:false,
  deepseek:{baseUrl:'https://api.deepseek.com',apiKey:'',model:'deepseek-v4-pro'},
  bailian:{baseUrl:'https://dashscope.aliyuncs.com/compatible-mode/v1',apiKey:'',model:'qwen3.5-ocr',workspaceId:''},
})
const session=ref<Session>({})
const capabilities=ref<Capabilities>({chat:false,pdf:false})
const messages=ref<ChatMessage[]>([])
const chatInput=ref(''), chatBusy=ref(false), chatError=ref('')
const channelKey=ref(''), targetMode=ref<'create'|'update'>('create'), targetDeviceKey=ref('')
const pdfFile=ref<File>(), consent=ref(localStorage.getItem('gateway-ai-data-consent')==='true')
const job=ref<Job>(), draft=ref<Draft>(), pdfError=ref(''), applying=ref(false), appliedMessage=ref('')
let pollTimer=0

const channels=computed<any[]>(()=>Array.isArray(store.value.channels)?store.value.channels.filter((item:any)=>item.enabled!==false&&item.role!=='forward'):[])
const devices=computed<any[]>(()=>Array.isArray(store.value.devices)?store.value.devices.filter((item:any)=>item.channelKey===channelKey.value):[])
const selectedChannel=computed(()=>channels.value.find(item=>item.channelKey===channelKey.value))
const pointColumns=computed(()=>{
  const protocol=selectedChannel.value?.protocol
  if(protocol==='iec104')return['ioa','pointType','dataType']
  if(protocol==='siemens-s7')return['area','dbNumber','register','dataType']
  if(protocol==='opcua')return['nodeId','dataType']
  if(protocol==='iec61850')return['objectRef','fc','dataType']
  return['function','register','quantity','dataType']
})
const canGenerate=computed(()=>Boolean(channelKey.value&&pdfFile.value&&consent.value&&!job.value?.status?.match(/queued|running/)))
const canManageAI=computed(()=>session.value.permissions?.some(item=>item==='*'||item==='ai.manage')===true)
const deepseekConfigured=computed(()=>Boolean(settings.deepseek.apiKey&&settings.deepseek.apiKey!==''))
const bailianConfigured=computed(()=>Boolean(settings.bailian.apiKey&&settings.bailian.workspaceId))
const chatReady=computed(()=>settings.enabled&&(capabilities.value.chat||deepseekConfigured.value))
const pdfReady=computed(()=>settings.enabled&&(capabilities.value.pdf||(deepseekConfigured.value&&bailianConfigured.value)))
const stageName=(value:string)=>({uploaded:'已上传',ocr:'PDF 解析',generating:'配置生成',validating:'规则校验',completed:'已完成',failed:'失败',cancelled:'已取消'} as Record<string,string>)[value]||value

async function loadSettings(){
  try{
    const result=await api<{settings:Settings;lastError?:string;capabilities?:Capabilities}>('/api/ai/settings')
    Object.assign(settings,result.settings)
    capabilities.value=result.capabilities||{chat:false,pdf:false}
    lastError.value=result.lastError||''
  }catch(cause){settingsError.value=cause instanceof Error?cause.message:'AI 设置读取失败'}
}
async function saveSettings(){
  settingsSaving.value=true;settingsError.value=''
  try{
    await api('/api/ai/settings',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(settings)})
    settingsOpen.value=false
    await loadSettings()
  }catch(cause){settingsError.value=cause instanceof Error?cause.message:'AI 设置保存失败'}
  finally{settingsSaving.value=false}
}
async function testProvider(provider:'deepseek'|'bailian'){
  settingsError.value=''
  try{
    await api('/api/ai/settings/test',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({provider,settings}),timeoutMs:25000})
    settingsError.value=(provider==='deepseek'?'DeepSeek':'百炼')+' 连接成功'
  }catch(cause){settingsError.value=cause instanceof Error?cause.message:'连接检测失败'}
}

function parseDeepSeekEvent(line:string){
  if(!line.startsWith('data:'))return{content:''}
  const data=line.slice(5).trim()
  if(!data||data==='[DONE]')return{content:''}
  try{
    const parsed=JSON.parse(data)
    return{
      content:parsed?.choices?.[0]?.delta?.content||'',
      action:parsed?.gatewayAction as GatewayAction|undefined,
      question:parsed?.gatewayQuestion as GatewayQuestion|undefined,
      form:parsed?.gatewayForm as GatewayForm|undefined,
    }
  }catch{return{content:''}}
}
async function runChat(content:string,hiddenUser=false){
  if(!content||chatBusy.value)return
  messages.value.push({role:'user',content,hidden:hiddenUser})
  chatBusy.value=true;chatError.value=''
  const assistant:ChatMessage={role:'assistant',content:''}
  messages.value.push(assistant)
  try{
    const response=await fetch('/api/ai/chat',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({messages:messages.value.slice(0,-1).slice(-20)})})
    if(!response.ok)throw new Error((await response.text()).trim()||'AI 请求失败')
    if(!response.body)throw new Error('浏览器不支持流式响应')
    const reader=response.body.getReader(), decoder=new TextDecoder()
    let pending=''
    while(true){
      const {done,value}=await reader.read()
      pending+=decoder.decode(value||new Uint8Array(),{stream:!done})
      const lines=pending.split(/\r?\n/);pending=lines.pop()||''
      for(const line of lines){
        const event=parseDeepSeekEvent(line)
        assistant.content+=event.content
        if(event.action)assistant.action={...event.action,state:'pending',selectedKeys:(event.action.arguments?.candidatePoints||[]).map((item:any)=>item.metric)}
        if(event.question)assistant.question={...event.question,selected:'',custom:''}
        if(event.form)assistant.form={...event.form,fields:(event.form.fields||[]).map(field=>({...field,value:field.value||''}))}
      }
      if(done)break
    }
    if(pending){
      const event=parseDeepSeekEvent(pending)
      assistant.content+=event.content
      if(event.action)assistant.action={...event.action,state:'pending',selectedKeys:(event.action.arguments?.candidatePoints||[]).map((item:any)=>item.metric)}
      if(event.question)assistant.question={...event.question,selected:'',custom:''}
      if(event.form)assistant.form={...event.form,fields:(event.form.fields||[]).map(field=>({...field,value:field.value||''}))}
    }
    if(!assistant.content)assistant.content=(assistant.question||assistant.form)?'继续操作前需要你确认参数。':'模型没有返回内容，请稍后重试。'
  }catch(cause){
    messages.value.pop()
    chatError.value=cause instanceof Error?cause.message:'AI 对话失败'
  }finally{chatBusy.value=false}
}
async function sendChat(){
  const content=chatInput.value.trim()
  if(!content||chatBusy.value)return
  chatInput.value=''
  await runChat(content)
}
function handleChatKeydown(event:KeyboardEvent){
  if(event.isComposing||event.key!=='Enter'||event.shiftKey)return
  event.preventDefault()
  sendChat()
}
async function applyChatAction(message:ChatMessage){
  const action=message.action
  if(!action||action.state==='applying'||action.state==='applied')return
  action.state='applying';action.error=''
  try{
    const result=await api<{message?:string;summary?:string}>(`/api/ai/chat-actions/${action.id}`,{
      method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({selectedKeys:action.selectedKeys||[]}),timeoutMs:60000,
    })
    action.state='applied'
    const executionSummary=result.summary||result.message||action.summary||'网关操作已执行'
    message.content+=(message.content?'\n\n':'')+`执行结果：${executionSummary}`
    await store.load()
    await runChat(
      `刚才的待确认操作已经成功执行：${executionSummary}。请重新读取当前网关配置，并结合本轮对话中用户最初的完整要求继续处理下一项尚未完成的操作。不要重复已经完成的步骤；如果所有步骤都已完成，请明确回复任务已经全部完成。`,
      true,
    )
  }catch(cause){
    action.state='failed'
    action.error=cause instanceof Error?cause.message:'网关操作执行失败'
  }
}
function clearChat(){messages.value=[];chatError.value=''}
function toggleActionCandidate(action:GatewayAction,key:string){
  const selected=new Set(action.selectedKeys||[])
  selected.has(key)?selected.delete(key):selected.add(key)
  action.selectedKeys=[...selected]
}
function toggleAllActionCandidates(action:GatewayAction){
  const points=action.arguments?.candidatePoints||[]
  action.selectedKeys=(action.selectedKeys?.length===points.length)?[]:points.map((item:any)=>item.metric)
}
async function submitGatewayQuestion(message:ChatMessage){
  const question=message.question
  if(!question||question.answered||chatBusy.value)return
  const answer=(question.custom||'').trim()||question.selected||''
  if(!answer){question.error='请选择一个选项或填写内容';return}
  question.error=''
  question.answered=true
  await runChat(
    `针对问题“${question.question}”，用户确认的答案是：“${answer}”。请使用这个答案继续完成原始网关任务；如果还缺少其他关键参数，继续使用结构化确认问题，不要自行猜测。`,
    true,
  )
}
async function submitGatewayForm(message:ChatMessage){
  const form=message.form
  if(!form||form.submitted||chatBusy.value)return
  const missing=form.fields.filter(field=>field.required&&!String(field.value||'').trim())
  if(missing.length){form.error=`请填写：${missing.map(field=>field.label).join('、')}`;return}
  form.error=''
  const values=Object.fromEntries(form.fields.map(field=>[field.key,field.value]))
  form.submitted=true
  await runChat(
    `用户已经确认“${form.title}”参数，结构化结果如下：${JSON.stringify(values)}。请使用这些参数继续完成原始网关任务，不要再次询问已经提供的字段；先生成完整执行计划和待确认操作。`,
    true,
  )
}

function selectPDF(event:Event){
  const input=event.target as HTMLInputElement
  const file=input.files?.[0]
  pdfError.value='';draft.value=undefined;job.value=undefined;appliedMessage.value=''
  if(!file){pdfFile.value=undefined;return}
  if(file.size>25*1024*1024){pdfError.value='PDF 不能超过 25 MB';input.value='';return}
  pdfFile.value=file
}
function rememberConsent(){if(consent.value)localStorage.setItem('gateway-ai-data-consent','true')}
async function createJob(){
  if(!canGenerate.value||!pdfFile.value)return
  rememberConsent();pdfError.value='';draft.value=undefined;appliedMessage.value=''
  const form=new FormData()
  form.append('file',pdfFile.value)
  form.append('channelKey',channelKey.value)
  form.append('targetMode',targetMode.value)
  if(targetMode.value==='update')form.append('targetDeviceKey',targetDeviceKey.value)
  try{
    job.value=await api<Job>('/api/ai/config-jobs',{method:'POST',body:form,timeoutMs:60000})
    schedulePoll()
  }catch(cause){pdfError.value=cause instanceof Error?cause.message:'PDF 任务创建失败'}
}
function schedulePoll(){window.clearTimeout(pollTimer);pollTimer=window.setTimeout(pollJob,1000)}
async function pollJob(){
  if(!job.value)return
  try{
    job.value=await api<Job>(`/api/ai/config-jobs/${job.value.id}`,{timeoutMs:15000})
    if(job.value.draft)draft.value=job.value.draft
    if(['queued','running'].includes(job.value.status))schedulePoll()
    else if(job.value.error)pdfError.value=job.value.error
  }catch(cause){pdfError.value=cause instanceof Error?cause.message:'任务状态读取失败'}
}
async function cancelJob(){
  if(!job.value)return
  try{await api(`/api/ai/config-jobs/${job.value.id}`,{method:'DELETE'});await pollJob()}catch{}
}
function removePoint(index:number){draft.value?.device.points.splice(index,1)}
function addPoint(){
  if(!draft.value)return
  draft.value.device.points.push({name:'新点位',metric:'',function:0,register:0,quantity:1,dataType:'',scale:1,offset:0,decimals:0})
}
async function applyDraft(){
  if(!draft.value)return
  applying.value=true;pdfError.value=''
  try{
    const response=await fetch(`/api/ai/config-drafts/${draft.value.id}/apply`,{
      method:'POST',headers:{'Content-Type':'application/json'},
      body:JSON.stringify({baseRevision:draft.value.baseRevision,device:draft.value.device}),
    })
    const raw=await response.text()
    let result:any={}
    try{result=raw?JSON.parse(raw):{}}catch{result={error:raw}}
    if(!response.ok){
      if(Array.isArray(result.blockingIssues))draft.value.blockingIssues=result.blockingIssues
      if(Array.isArray(result.warnings))draft.value.warnings=result.warnings
      throw new Error(result.error||raw||`请求失败 (${response.status})`)
    }
    appliedMessage.value=`设备 ${result.deviceKey} 已应用，共 ${result.points} 个点位`
    await store.load()
    draft.value=undefined;job.value=undefined
  }catch(cause){
    const message=cause instanceof Error?cause.message:'AI 草稿应用失败'
    pdfError.value=message
  }finally{applying.value=false}
}

onMounted(async()=>{
  if(!Object.keys(store.value).length)await store.load().catch(()=>{})
  session.value=await api<Session>('/api/session').catch(()=>({}))
  await loadSettings()
  channelKey.value=channels.value[0]?.channelKey||''
})
onBeforeUnmount(()=>window.clearTimeout(pollTimer))
</script>

<template>
  <section class="view-stack ai-view">
    <section class="content-card ai-head">
      <div>
        <span class="eyebrow">AI ASSISTANT</span>
        <h2>AI 助手</h2>
        <p>使用 DeepSeek V4 分析网关配置与运行状态，通过百炼识别设备 PDF 并生成安全草稿。</p>
      </div>
      <div class="ai-head-actions">
        <span class="state-tag" :class="chatReady?'ok':'warn'">{{chatReady?'对话可用':'未启用'}}</span>
        <button v-if="canManageAI" @click="settingsOpen=true">模型设置</button>
      </div>
    </section>

    <section class="content-card ai-workspace">
      <div class="ai-tabs"><button :class="{active:tab==='chat'}" @click="tab='chat'">智能对话</button><button :class="{active:tab==='pdf'}" @click="tab='pdf'">PDF 生成配置</button></div>

      <template v-if="tab==='chat'">
        <div class="ai-chat">
          <div class="ai-chat-toolbar"><span>网关专用助手 · 只读取脱敏配置、状态摘要和最近错误</span><button :disabled="!messages.length" @click="clearChat">清空对话</button></div>
          <div class="ai-messages">
            <div v-if="!messages.length" class="ai-welcome">
              <strong>可以这样问我</strong>
              <button @click="chatInput='当前有哪些异常点位，应该如何排查？';sendChat()">分析异常点位</button>
              <button @click="chatInput='检查当前采集配置是否存在不合理项';sendChat()">检查采集配置</button>
              <button @click="chatInput='解释当前网关运行负载和连接状态';sendChat()">解释运行状态</button>
            </div>
            <article v-for="(message,index) in messages" :key="index" :class="['ai-message',message.role,{hidden:message.hidden}]">
              <span>{{message.role==='user'?'我':'AI'}}</span><div>{{message.content}}<i v-if="chatBusy&&index===messages.length-1"/>
                <section v-if="message.action" class="ai-action-card" :class="message.action.state">
                  <div class="ai-action-title"><b>{{message.action.title}}</b><em>{{message.action.state==='applied'?'已执行':message.action.state==='applying'?'执行中':message.action.state==='failed'?'执行失败':message.action.state==='cancelled'?'已取消':'等待确认'}}</em></div>
                  <p>{{message.action.summary}}</p>
                  <ol v-if="message.action.steps?.length" class="ai-plan-steps">
                    <li v-for="(step,stepIndex) in message.action.steps" :key="step"><i>{{stepIndex+1}}</i><span>{{step}}</span><em>{{message.action.state==='applied'?'已完成':message.action.state==='applying'&&stepIndex===0?'执行中':'等待执行'}}</em></li>
                  </ol>
                  <div v-if="message.action.tool==='browse_and_add_points'" class="ai-candidate-picker">
                    <div><b>候选点位</b><button @click="toggleAllActionCandidates(message.action)">{{message.action.selectedKeys?.length===message.action.arguments.candidatePoints?.length?'取消全选':'全选'}}</button><span>已选 {{message.action.selectedKeys?.length||0}} / {{message.action.arguments.candidatePoints?.length||0}}</span></div>
                    <label v-for="point in message.action.arguments.candidatePoints||[]" :key="point.metric">
                      <input type="checkbox" :checked="message.action.selectedKeys?.includes(point.metric)" @change="toggleActionCandidate(message.action,point.metric)"/>
                      <span><strong>{{point.name||point.metric}}</strong><small>{{point.nodeId||point.objectRef||`${point.area||''}${point.dbNumber?point.dbNumber+'.':''}${point.register??''}`}}</small></span>
                      <code>{{point.metric}}</code>
                    </label>
                  </div>
                  <small>配置变更将执行版本校验、完整校验和热加载；失败时自动恢复。</small>
                  <p v-if="message.action.error" class="ai-action-error">{{message.action.error}}</p>
                  <div v-if="!['applied','cancelled'].includes(message.action.state||'pending')" class="ai-action-buttons"><button :disabled="message.action.state==='applying'" @click="message.action.state='cancelled';message.action.error='已取消，本次操作不会修改网关'">取消</button><button class="primary" :disabled="message.action.state==='applying'||(message.action.tool==='browse_and_add_points'&&!message.action.selectedKeys?.length)" @click="applyChatAction(message)">{{message.action.state==='applying'?'正在执行':'确认执行'}}</button></div>
                </section>
                <section v-if="message.question" class="ai-question-card" :class="{answered:message.question.answered}">
                  <div class="ai-question-head"><b>{{message.question.question}}</b><em>{{message.question.answered?'已回答':'等待选择'}}</em></div>
                  <p v-if="message.question.reason">{{message.question.reason}}</p>
                  <div class="ai-question-options">
                    <button v-for="option in message.question.options" :key="option" :class="{active:message.question.selected===option&&!message.question.custom}" :disabled="message.question.answered" @click="message.question.selected=option;message.question.custom=''">{{option}}</button>
                  </div>
                  <input v-if="message.question.allowCustom" v-model="message.question.custom" :disabled="message.question.answered" placeholder="也可以填写自定义内容" @input="message.question.selected=''"/>
                  <p v-if="message.question.error" class="ai-action-error">{{message.question.error}}</p>
                  <div class="ai-action-buttons"><span v-if="message.question.answered">已选择：{{message.question.custom||message.question.selected}}</span><button v-else class="primary" :disabled="chatBusy" @click="submitGatewayQuestion(message)">确认并继续</button></div>
                </section>
                <section v-if="message.form" class="ai-form-card" :class="{submitted:message.form.submitted}">
                  <div class="ai-question-head"><b>{{message.form.title}}</b><em>{{message.form.submitted?'已提交':'补充参数'}}</em></div>
                  <p v-if="message.form.reason">{{message.form.reason}}</p>
                  <div class="ai-inline-form">
                    <label v-for="field in message.form.fields" :key="field.key"><span>{{field.label}}<b v-if="field.required">*</b></span>
                      <select v-if="field.type==='select'" v-model="field.value" :disabled="message.form.submitted"><option value="">请选择</option><option v-for="option in field.options||[]" :key="option">{{option}}</option></select>
                      <input v-else v-model="field.value" :type="field.type==='number'?'number':'text'" :disabled="message.form.submitted" :placeholder="field.placeholder||''"/>
                    </label>
                  </div>
                  <p v-if="message.form.error" class="ai-action-error">{{message.form.error}}</p>
                  <div class="ai-action-buttons"><span v-if="message.form.submitted">参数已提交，正在继续生成计划</span><button v-else class="primary" :disabled="chatBusy" @click="submitGatewayForm(message)">确认参数并继续</button></div>
                </section>
              </div>
            </article>
          </div>
          <p v-if="chatError" class="notice bad">{{chatError}}</p>
          <div class="ai-composer"><textarea v-model="chatInput" :disabled="chatBusy||!chatReady" rows="3" placeholder="询问网关配置、协议、采集、转发或运行异常；Enter 发送，Shift+Enter 换行" @keydown="handleChatKeydown"/><button class="primary" :disabled="chatBusy||!chatInput.trim()||!chatReady" @click="sendChat">{{chatBusy?'分析中':'发送'}}</button></div>
        </div>
      </template>

      <template v-else>
        <div class="ai-pdf-layout">
          <section class="ai-pdf-setup">
            <h3>1. 选择目标并上传文档</h3>
            <div class="ai-form-grid">
              <label>采集通道<select v-model="channelKey"><option v-for="channel in channels" :key="channel.channelKey" :value="channel.channelKey">{{channel.name}} · {{channel.protocol}}</option></select></label>
              <label>生成方式<select v-model="targetMode"><option value="create">新增设备</option><option value="update">更新已有设备</option></select></label>
              <label v-if="targetMode==='update'">目标设备<select v-model="targetDeviceKey"><option value="">请选择设备</option><option v-for="device in devices" :key="device.deviceKey" :value="device.deviceKey">{{device.name}} ({{device.deviceKey}})</option></select></label>
              <label class="span-2">设备 PDF<input type="file" accept=".pdf,application/pdf" @change="selectPDF"/><small>{{pdfFile?`${pdfFile.name} · ${(pdfFile.size/1024/1024).toFixed(1)} MB`:'最大 25 MB，支持文本、扫描件和表格'}}</small></label>
            </div>
            <label class="ai-consent"><input v-model="consent" type="checkbox"/><span>我确认：PDF 将上传至阿里云百炼临时存储，解析内容将发送至 DeepSeek 用于生成配置；网关本地文件会在任务结束后立即删除，百炼临时文件最长保留 48 小时后自动失效。</span></label>
            <p v-if="!pdfReady" class="ai-capability-note">PDF 生成配置尚不可用，请在模型设置中补充百炼 API Key 和 Workspace ID；智能对话不受影响。</p>
            <div class="action-row"><button class="primary" :disabled="!pdfReady||!canGenerate||(targetMode==='update'&&!targetDeviceKey)" @click="createJob">生成配置草稿</button><button v-if="job&&['queued','running'].includes(job.status)" @click="cancelJob">取消任务</button></div>
            <div v-if="job" class="ai-progress"><div><span>{{stageName(job.stage)}}</span><strong>{{job.progress}}%</strong></div><i><b :style="{width:job.progress+'%'}"/></i><p>{{job.message}}</p></div>
            <p v-if="pdfError" class="notice bad">{{pdfError}}</p>
            <p v-if="appliedMessage" class="notice ai-success">{{appliedMessage}}</p>
          </section>

          <section v-if="draft" class="ai-draft">
            <div class="ai-draft-head"><div><h3>2. 检查配置草稿</h3><p>确认模型识别结果，缺失字段修正后才能应用。</p></div><span>{{draft.device.points?.length||0}} 个点位</span></div>
            <div v-if="draft.blockingIssues?.length" class="ai-issues blocking"><strong>生成时发现的待确认项（修正字段后可重新尝试应用）</strong><span v-for="item in draft.blockingIssues" :key="item">{{item}}</span></div>
            <div v-if="draft.warnings?.length" class="ai-issues"><strong>生成提示</strong><span v-for="item in draft.warnings" :key="item">{{item}}</span></div>
            <div class="ai-device-grid">
              <label>设备标识<input v-model.trim="draft.device.deviceKey"/></label><label>设备名称<input v-model.trim="draft.device.name"/></label>
              <label>设备地址<input v-model.trim="draft.device.address"/></label>
              <label v-if="selectedChannel?.protocol?.startsWith('modbus')">从站 ID<input v-model.number="draft.device.slaveId" type="number" min="1" max="247"/></label>
              <label v-if="selectedChannel?.protocol==='iec104'">公共地址<input v-model.number="draft.device.commonAddress" type="number" min="1"/></label>
            </div>
            <div class="ai-point-toolbar"><strong>点位配置</strong><button @click="addPoint">新增点位</button></div>
            <div class="table-scroll ai-point-table"><table><thead><tr><th>#</th><th>名称</th><th>标识符</th><th v-for="column in pointColumns" :key="column">{{column}}</th><th>单位</th><th>倍率</th><th>小数位</th><th>操作</th></tr></thead>
              <tbody><tr v-for="(point,index) in draft.device.points" :key="index"><td>{{index+1}}</td><td><input v-model.trim="point.name"/></td><td><input v-model.trim="point.metric"/></td>
                <td v-for="column in pointColumns" :key="column"><input v-if="!['dataType','pointType','area','fc'].includes(column)" v-model="point[column]" :type="['function','register','quantity','ioa','dbNumber'].includes(column)?'number':'text'"/><input v-else v-model.trim="point[column]"/></td>
                <td><input v-model.trim="point.unit"/></td><td><input v-model.number="point.scale" type="number" step="any"/></td><td><input v-model.number="point.decimals" type="number" min="0"/></td><td><button class="danger" @click="removePoint(index)">移除</button></td></tr></tbody>
            </table></div>
            <div class="ai-apply"><span>应用前将再次执行完整配置校验，并在热加载失败时自动恢复。</span><button class="primary" :disabled="applying" @click="applyDraft">{{applying?'正在应用':'校验并应用配置'}}</button></div>
          </section>
        </div>
      </template>
    </section>

    <div v-if="settingsOpen" class="ai-settings-modal">
      <div class="dialog-panel ai-settings-dialog">
        <div class="ai-dialog-head"><div><span class="eyebrow">MODEL SETTINGS</span><h3>AI 模型设置</h3><p>只配置 DeepSeek 即可使用智能对话；百炼仅用于 PDF 解析。</p></div><button aria-label="关闭" @click="settingsOpen=false">×</button></div>
        <div class="ai-enable"><div><strong>启用网关 AI 助手</strong><span>仅回答和处理当前工业网关相关问题</span></div><label class="ai-switch"><input v-model="settings.enabled" type="checkbox"/><i/></label></div>
        <div class="ai-provider-list">
          <section class="ai-provider-card">
            <div class="ai-provider-head"><div><b>01</b><span><strong>DeepSeek V4</strong><small>智能对话 · 配置分析 · 草稿生成</small></span></div><em :class="{ready:deepseekConfigured}">{{deepseekConfigured?'已配置':'必需'}}</em></div>
            <div class="ai-form-grid"><label>API 地址<input v-model.trim="settings.deepseek.baseUrl"/></label><label>模型<input v-model.trim="settings.deepseek.model"/></label><label class="span-2">API Key<input v-model="settings.deepseek.apiKey" type="password" autocomplete="new-password" placeholder="输入后将加密保存"/></label></div>
            <div class="ai-provider-actions"><span>配置完成后即可单独使用对话功能</span><button @click="testProvider('deepseek')">检测连接</button></div>
          </section>
          <section class="ai-provider-card optional">
            <div class="ai-provider-head"><div><b>02</b><span><strong>阿里云百炼</strong><small>PDF 文档解析 · 扫描页与表格识别</small></span></div><em :class="{ready:bailianConfigured}">{{bailianConfigured?'已配置':'可选'}}</em></div>
            <div class="ai-form-grid"><label>API 地址<input v-model.trim="settings.bailian.baseUrl"/></label><label>Workspace ID<input v-model.trim="settings.bailian.workspaceId" placeholder="PDF 功能必填"/></label><label>模型<input v-model="settings.bailian.model" disabled/></label><label>API Key<input v-model="settings.bailian.apiKey" type="password" autocomplete="new-password" placeholder="PDF 功能必填"/></label></div>
            <div class="ai-provider-actions"><span>不配置时仅关闭 PDF 功能，不影响对话</span><button @click="testProvider('bailian')">检测连接</button></div>
          </section>
        </div>
        <p v-if="lastError" class="notice bad">最近错误：{{lastError}}</p><p v-if="settingsError" class="notice" :class="settingsError.includes('成功')?'ai-success':'bad'">{{settingsError}}</p>
        <div class="dialog-actions"><button @click="settingsOpen=false">取消</button><button class="primary" :disabled="settingsSaving" @click="saveSettings">{{settingsSaving?'保存中':'保存设置'}}</button></div>
      </div>
    </div>
  </section>
</template>
