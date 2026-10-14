<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import pdfWorkerUrl from 'pdfjs-dist/build/pdf.worker.min.mjs?url'
import {
  IconActivity,
  IconChartLine,
  IconCpu,
  IconFileDescription,
  IconLink,
  IconMessageChatbot,
  IconPointFilled,
  IconSend2,
  IconSettings,
  IconStack2,
  IconTrash,
} from '@tabler/icons-vue'
import { api } from '../api'
import {useFormDraftGuard} from '../composables/useFormDraftGuard.ts'
import { useConfigStore } from '../stores/config'

type Provider = { baseUrl:string; apiKey:string; model:string }
type Settings = { enabled:boolean; deepseek:Provider }
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
  deepseek:{baseUrl:'https://api.deepseek.com',apiKey:'',model:'deepseek-v4-flash-vision-exp'},
})
const settingsDraft=useFormDraftGuard(()=>settings,value=>Object.assign(settings,value),saveSettings)
async function closeSettings(){if(await settingsDraft.requestLeave())settingsOpen.value=false}
defineExpose({requestLeave:settingsDraft.requestLeave})
const session=ref<Session>({})
const capabilities=ref<Capabilities>({chat:false,pdf:false})
const messages=ref<ChatMessage[]>([])
const chatInput=ref(''), chatBusy=ref(false), chatError=ref('')
const chatPhase=ref('')
const channelKey=ref(''), targetMode=ref<'create'|'update'>('create'), targetDeviceKey=ref('')
const pdfFile=ref<File>(), consent=ref(localStorage.getItem('gateway-ai-data-consent')==='true')
const job=ref<Job>(), draft=ref<Draft>(), pdfError=ref(''), preparingPDF=ref(false), applying=ref(false), appliedMessage=ref('')
let pollTimer=0

const channels=computed<any[]>(()=>Array.isArray(store.value.channels)?store.value.channels.filter((item:any)=>item.enabled!==false&&item.role!=='forward'&&item.protocol!=='opcda'):[])
const devices=computed<any[]>(()=>Array.isArray(store.value.devices)?store.value.devices.filter((item:any)=>item.channelKey===channelKey.value):[])
const selectedChannel=computed(()=>channels.value.find(item=>item.channelKey===channelKey.value))
const pointColumns=computed(()=>{
  const protocol=selectedChannel.value?.protocol
  if(protocol==='iec104')return['ioa','pointType','dataType']
  if(protocol==='siemens-s7')return['area','dbNumber','register','dataType']
  if(protocol==='opcua')return['nodeId','dataType']
  if(protocol==='iec61850')return['objectRef','fc','dataType']
  if(protocol==='iec61850-goose')return['goCbRef','dataSetRef','gooseIndex','dataType']
  return['function','register','quantity','dataType']
})
const protocolHint=computed(()=>{
  const protocol=selectedChannel.value?.protocol
  if(protocol==='modbus-tcp'||protocol==='modbus-rtu')return'将识别从站号、功能码、寄存器、数据类型、字节序、倍率和单位。'
  if(protocol==='iec104')return'将识别公共地址、IOA、点位类型、数据类型、倍率和单位。'
  if(protocol==='siemens-s7')return'将识别 PLC 型号、机架/槽位、存储区、DB、偏移和数据类型。'
  if(protocol==='iec61850')return'将识别 IED、对象引用、FC、数据类型和单位。'
  if(protocol==='iec61850-goose')return'将识别 GoCBRef、DataSet、成员序号和网络参数。'
  if(protocol==='opcua')return'将识别端点、NodeId 和数据类型；在线服务器仍优先使用一键浏览。'
  return'模型只会生成目标协议对应的字段，缺失参数将标记为待确认。'
})
const canGenerate=computed(()=>Boolean(channelKey.value&&pdfFile.value&&consent.value&&!preparingPDF.value&&!job.value?.status?.match(/queued|running/)))
const canManageAI=computed(()=>session.value.permissions?.some(item=>item==='*'||item==='ai.manage')===true)
const deepseekConfigured=computed(()=>Boolean(settings.deepseek.apiKey&&settings.deepseek.apiKey!==''))
const chatReady=computed(()=>settings.enabled&&(capabilities.value.chat||deepseekConfigured.value))
const pdfReady=computed(()=>settings.enabled&&(capabilities.value.pdf||deepseekConfigured.value))
const stageName=(value:string)=>({uploaded:'已上传',extracting:'PDF 预处理',ocr:'PDF 解析',generating:'配置生成',validating:'规则校验',completed:'已完成',failed:'失败',cancelled:'已取消'} as Record<string,string>)[value]||value

async function loadSettings(){
  try{
    const result=await api<{settings:Settings;lastError?:string;capabilities?:Capabilities}>('/api/ai/settings')
    Object.assign(settings,result.settings)
    settingsDraft.markSaved()
    capabilities.value=result.capabilities||{chat:false,pdf:false}
    lastError.value=result.lastError||''
  }catch(cause){settingsError.value=cause instanceof Error?cause.message:'AI 设置读取失败'}
}
async function saveSettings(){
  settingsSaving.value=true;settingsError.value=''
  try{
    await api('/api/ai/settings',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(settings)})
    settingsDraft.markSaved()
    settingsOpen.value=false
    await loadSettings()
  }catch(cause){settingsError.value=cause instanceof Error?cause.message:'AI 设置保存失败'}
  finally{settingsSaving.value=false}
}
async function testProvider(provider:'deepseek'){
  settingsError.value=''
  try{
    await api('/api/ai/settings/test',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({provider,settings}),timeoutMs:25000})
    settingsError.value='DeepSeek 连接成功'
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
  chatBusy.value=true;chatError.value='';chatPhase.value='正在读取网关配置与运行状态'
  const assistant:ChatMessage={role:'assistant',content:''}
  messages.value.push(assistant)
  try{
    window.setTimeout(()=>{if(chatBusy.value)chatPhase.value='正在分析目标并规划下一步'},450)
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
  }finally{chatBusy.value=false;chatPhase.value=''}
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
    const result=await api<{message?:string;summary?:string;tool?:string;arguments?:Record<string,any>;verification?:Record<string,any>}>(`/api/ai/chat-actions/${action.id}`,{
      method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({selectedKeys:action.selectedKeys||[]}),timeoutMs:60000,
    })
    action.state='applied'
    const executionSummary=result.summary||result.message||action.summary||'网关操作已执行'
    message.content+=(message.content?'\n\n':'')+`执行结果：${executionSummary}`
    await store.load()
    const executionContext={
      operation:result.tool||action.tool,
      parameters:result.arguments||action.arguments,
      result:result.verification||{verified:true,message:executionSummary},
    }
    await runChat(
      `网关已执行并验证上一项操作。执行上下文：${JSON.stringify(executionContext)}。请重新读取当前网关配置，继续完成本轮对话里尚未完成的目标。不要重复已完成步骤；如果全部完成，请明确总结已完成内容和验证结果。`,
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
    `结构化确认结果：${JSON.stringify({question:question.question,answer})}。请把答案合并到当前未完成的网关任务中并继续执行；不要重复询问已经确认的信息。`,
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
    `结构化参数已确认：${JSON.stringify({form:form.title,values})}。请把这些参数合并到当前未完成的网关任务中，直接生成下一项安全操作；不要再次询问已经提供的字段。`,
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

async function canvasJPEG(canvas:HTMLCanvasElement):Promise<Blob>{
  return new Promise((resolve,reject)=>canvas.toBlob(blob=>blob?resolve(blob):reject(new Error('PDF 页面图像生成失败')),'image/jpeg',0.84))
}

async function preprocessPDF(file:File):Promise<{text:string;pages:Blob[]}>{
	const {getDocument,GlobalWorkerOptions}=await import('pdfjs-dist')
	GlobalWorkerOptions.workerSrc=pdfWorkerUrl
  const loadingTask=getDocument({data:new Uint8Array(await file.arrayBuffer())})
  const pdfDocument=await loadingTask.promise
  try{
    if(pdfDocument.numPages>40)throw new Error(`PDF 共 ${pdfDocument.numPages} 页，当前最多处理 40 页，请拆分后重试`)
    const textParts:string[]=[]
    const pages:Blob[]=[]
    let imageBytes=0
    for(let pageNumber=1;pageNumber<=pdfDocument.numPages;pageNumber++){
      const page=await pdfDocument.getPage(pageNumber)
      const content=await page.getTextContent()
      const pageText=(content.items as any[]).map(item=>typeof item?.str==='string'?item.str:'').join(' ').trim()
      const usefulCharacters=(pageText.match(/[\p{L}\p{N}]/gu)||[]).length
      if(pageText)textParts.push(`\n--- PDF 第 ${pageNumber} 页 ---\n${pageText}`)
      if(usefulCharacters>=24)continue
      const baseViewport=page.getViewport({scale:1})
      const scale=Math.min(1.6,1600/Math.max(1,baseViewport.width))
      const viewport=page.getViewport({scale})
      const canvas=window.document.createElement('canvas')
      canvas.width=Math.ceil(viewport.width)
      canvas.height=Math.ceil(viewport.height)
      const context=canvas.getContext('2d',{alpha:false})
      if(!context)throw new Error('浏览器无法创建 PDF 页面画布')
      await page.render({canvas,canvasContext:context,viewport}).promise
      const blob=await canvasJPEG(canvas)
      imageBytes+=blob.size
      if(imageBytes>48*1024*1024)throw new Error('PDF 扫描页面转换后超过 48 MB，请拆分文档后重试')
      pages.push(blob)
      canvas.width=canvas.height=1
    }
    const text=textParts.join('').trim()
    if(!text&&pages.length===0)throw new Error('PDF 中没有识别到文字或可转换页面')
    return{text,pages}
  }finally{
    await loadingTask.destroy()
  }
}

async function createJob(){
  if(!canGenerate.value||!pdfFile.value)return
  rememberConsent();pdfError.value='';draft.value=undefined;appliedMessage.value='';preparingPDF.value=true
  try{
    const prepared=await preprocessPDF(pdfFile.value)
    const form=new FormData()
    form.append('sourceName',pdfFile.value.name)
    form.append('channelKey',channelKey.value)
    form.append('targetMode',targetMode.value)
    if(prepared.text)form.append('extractedText',prepared.text)
    prepared.pages.forEach((page,index)=>form.append('page',page,`page-${String(index+1).padStart(3,'0')}.jpg`))
    if(targetMode.value==='update')form.append('targetDeviceKey',targetDeviceKey.value)
    job.value=await api<Job>('/api/ai/config-jobs',{method:'POST',body:form,timeoutMs:60000})
    schedulePoll()
  }catch(cause){pdfError.value=cause instanceof Error?cause.message:'PDF 任务创建失败'}
  finally{preparingPDF.value=false}
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
      <div class="ai-head-copy">
        <span class="eyebrow">AI ASSISTANT</span>
        <h2>AI 助手</h2>
        <p>使用 DeepSeek 分析网关状态，并从厂家点表和协议文档生成可校验的设备与点位草稿。</p>
      </div>
      <div class="ai-head-actions">
        <span class="state-tag" :class="chatReady?'ok':'warn'"><IconPointFilled aria-hidden="true"/>{{chatReady?'对话可用':'未启用'}}</span>
        <button v-if="canManageAI" class="ai-icon-button" @click="settingsOpen=true"><IconSettings aria-hidden="true"/>模型设置</button>
      </div>
    </section>

    <section class="content-card ai-workspace">
      <div class="ai-tabs"><button :class="{active:tab==='chat'}" @click="tab='chat'"><IconMessageChatbot aria-hidden="true"/>智能对话</button><button :class="{active:tab==='pdf'}" @click="tab='pdf'"><IconFileDescription aria-hidden="true"/>PDF 生成配置</button></div>

      <template v-if="tab==='chat'">
        <div class="ai-chat">
          <div class="ai-chat-toolbar"><span>网关专用 Agent · 可分析状态、诊断连接，并通过确认卡安全执行配置操作</span><button :disabled="!messages.length" @click="clearChat"><IconTrash aria-hidden="true"/>清空对话</button></div>
          <div class="ai-messages">
            <div v-if="!messages.length" class="ai-welcome">
              <strong>可以这样问我</strong>
              <button @click="chatInput='当前有哪些异常点位，应该如何排查？';sendChat()"><IconActivity aria-hidden="true"/>分析异常点位</button>
              <button @click="chatInput='检查当前采集配置是否存在不合理项';sendChat()"><IconStack2 aria-hidden="true"/>检查采集配置</button>
              <button @click="chatInput='解释当前网关运行负载和连接状态';sendChat()"><IconChartLine aria-hidden="true"/>解释运行状态</button>
              <button @click="chatInput='帮我创建一个采集设备，缺少的参数请让我选择';sendChat()"><IconCpu aria-hidden="true"/>创建采集设备</button>
              <button @click="chatInput='诊断当前异常设备的连接并给出处理建议';sendChat()"><IconLink aria-hidden="true"/>自动诊断连接</button>
            </div>
            <article v-for="(message,index) in messages" :key="index" :class="['ai-message',message.role,{hidden:message.hidden}]">
              <span>{{message.role==='user'?'我':'AI'}}</span><div><template v-if="message.content">{{message.content}}</template><template v-else-if="chatBusy&&index===messages.length-1"><span class="ai-thinking-text">{{chatPhase}}</span></template><i v-if="chatBusy&&index===messages.length-1"/>
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
          <div class="ai-composer"><label class="ai-composer-field"><span>PROMPT · 输入指令</span><textarea v-model="chatInput" :disabled="chatBusy||!chatReady" rows="3" placeholder="询问网关配置、协议、采集、转发或运行异常；Enter 发送，Shift+Enter 换行" @keydown="handleChatKeydown"/></label><button class="primary" :disabled="chatBusy||!chatInput.trim()||!chatReady" @click="sendChat"><IconSend2 aria-hidden="true"/><span>{{chatBusy?'分析中':'发送'}}</span></button></div>
        </div>
      </template>

      <template v-else>
        <div class="ai-pdf-layout">
          <section class="ai-pdf-setup">
            <h3>1. 选择目标协议并上传文档</h3>
            <div class="ai-form-grid">
              <label>目标采集通道<select v-model="channelKey"><option v-for="channel in channels" :key="channel.channelKey" :value="channel.channelKey">{{channel.name}} · {{channel.protocol}}</option></select><small>{{protocolHint}}</small></label>
              <label>生成方式<select v-model="targetMode"><option value="create">新增设备</option><option value="update">更新已有设备</option></select></label>
              <label v-if="targetMode==='update'">目标设备<select v-model="targetDeviceKey"><option value="">请选择设备</option><option v-for="device in devices" :key="device.deviceKey" :value="device.deviceKey">{{device.name}} ({{device.deviceKey}})</option></select></label>
              <label class="span-2">设备 PDF<input type="file" accept=".pdf,application/pdf" @change="selectPDF"/><small>{{pdfFile?`${pdfFile.name} · ${(pdfFile.size/1024/1024).toFixed(1)} MB`:'最大 25 MB，支持厂家点表、通信规约和扫描件'}}</small></label>
            </div>
            <label class="ai-consent"><input v-model="consent" type="checkbox"/><span>我确认：网关会在本地提取 PDF 文字；扫描件仅把本地转换后的页面图像发送至 DeepSeek 识别，临时文件在任务结束后删除。生成结果必须检查确认后才会应用。</span></label>
            <p class="ai-capability-note">OPC DA 未列入目标通道：该协议应直接使用服务器一键浏览添加点位，无需从 PDF 建点。</p>
            <p v-if="!pdfReady" class="ai-capability-note">PDF 生成配置尚不可用，请先启用 AI 助手并配置 DeepSeek API Key。</p>
            <div class="action-row"><button class="primary" :disabled="!pdfReady||!canGenerate||(targetMode==='update'&&!targetDeviceKey)" @click="createJob">{{preparingPDF?'正在本地解析 PDF':'生成配置草稿'}}</button><button v-if="job&&['queued','running'].includes(job.status)" @click="cancelJob">取消任务</button></div>
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
              <label v-if="selectedChannel?.protocol==='siemens-s7'">PLC 型号<input v-model.trim="draft.device.plcModel"/></label>
              <label v-if="selectedChannel?.protocol==='siemens-s7'">机架<input v-model.number="draft.device.rack" type="number" min="0"/></label>
              <label v-if="selectedChannel?.protocol==='siemens-s7'">槽位<input v-model.number="draft.device.slot" type="number" min="0"/></label>
              <label v-if="selectedChannel?.protocol==='iec61850'">IED 名称<input v-model.trim="draft.device.iedName"/></label>
            </div>
            <div class="ai-point-toolbar"><strong>点位配置</strong><button @click="addPoint">新增点位</button></div>
            <div class="table-scroll ai-point-table"><table><thead><tr><th>#</th><th>名称</th><th>标识符</th><th v-for="column in pointColumns" :key="column">{{column}}</th><th>单位</th><th>倍率</th><th>小数位</th><th>操作</th></tr></thead>
              <tbody><tr v-for="(point,index) in draft.device.points" :key="index"><td>{{index+1}}</td><td><input v-model.trim="point.name"/></td><td><input v-model.trim="point.metric"/></td>
                <td v-for="column in pointColumns" :key="column"><input v-if="!['dataType','pointType','area','fc'].includes(column)" v-model="point[column]" :type="['function','register','quantity','ioa','dbNumber','gooseIndex'].includes(column)?'number':'text'"/><input v-else v-model.trim="point[column]"/></td>
                <td><input v-model.trim="point.unit"/></td><td><input v-model.number="point.scale" type="number" step="any"/></td><td><input v-model.number="point.decimals" type="number" min="0"/></td><td><button class="danger" @click="removePoint(index)">移除</button></td></tr></tbody>
            </table></div>
            <div class="ai-apply"><span>应用前将再次执行完整配置校验，并在热加载失败时自动恢复。</span><button class="primary" :disabled="applying" @click="applyDraft">{{applying?'正在应用':'校验并应用配置'}}</button></div>
          </section>
        </div>
      </template>
    </section>

    <div v-if="settingsOpen" class="ai-settings-modal">
      <div class="dialog-panel ai-settings-dialog">
        <div class="ai-dialog-head"><div><span class="eyebrow">MODEL SETTINGS</span><h3>AI 模型设置</h3><p>只需配置一个 DeepSeek 模型，即可使用智能对话和 PDF 配置生成。</p></div><button aria-label="关闭" @click="closeSettings">×</button></div>
        <div class="ai-enable"><div><strong>启用网关 AI 助手</strong><span>仅回答和处理当前工业网关相关问题</span></div><label class="ai-switch"><input v-model="settings.enabled" type="checkbox"/><i/></label></div>
        <div class="ai-provider-list">
          <section class="ai-provider-card">
            <div class="ai-provider-head"><div><b>01</b><span><strong>DeepSeek V4</strong><small>智能对话 · PDF 视觉识别 · 多协议草稿生成</small></span></div><em :class="{ready:deepseekConfigured}">{{deepseekConfigured?'已配置':'必需'}}</em></div>
            <div class="ai-form-grid"><label>API 地址<input v-model.trim="settings.deepseek.baseUrl"/></label><label>模型<input v-model.trim="settings.deepseek.model"/></label><label class="span-2">API Key<input v-model="settings.deepseek.apiKey" type="password" autocomplete="new-password" placeholder="输入后将加密保存"/></label></div>
            <div class="ai-provider-actions"><span>扫描型 PDF 需要支持图片输入的 DeepSeek 视觉模型；文字型 PDF 由网关本地提取。</span><button @click="testProvider('deepseek')">检测连接</button></div>
          </section>
        </div>
        <p v-if="lastError" class="notice bad">最近错误：{{lastError}}</p><p v-if="settingsError" class="notice" :class="settingsError.includes('成功')?'ai-success':'bad'">{{settingsError}}</p>
        <div class="dialog-actions"><button @click="closeSettings">取消</button><button class="primary" :disabled="settingsSaving" @click="saveSettings">{{settingsSaving?'保存中':'保存设置'}}</button></div>
      </div>
    </div>
  </section>
</template>
