<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import type { Ref } from 'vue'
import { api } from '../api'
import { confirmAction } from '../confirm'
import { useConfigStore } from '../stores/config'
import type { GatewayStatus, PointStatus } from '../types'

const store = useConfigStore()
const emit = defineEmits<{dirty:[value:boolean]}>()
const sharedStatus = inject<Ref<GatewayStatus | null>>('gatewayRealtimeStatus')
const status = computed(() => sharedStatus?.value || null)
const loading = ref(false)
const message = ref('')
const error = ref('')
const activeChannel = ref(0)
const mqttSwitchOpen = ref(false)
const pendingChannel = ref(-1)
const savedChannels = ref('')
const savedActivation = ref('')
const activation = reactive({ enabled:false, hardwareId:'', sn:'', broker:'', deviceSecret:'***' })
const activationFile = ref('activation.local.json')
const activationImport = ref<HTMLInputElement>()
const templateEditor = ref<HTMLTextAreaElement>()
const templateHighlight = ref<HTMLElement>()
const pointStatuses = ref<PointStatus[]>([])
const previewMode = ref<'template'|'realtime'>('template')
const suggestOpen = ref(false)
const suggestIndex = ref(0)
const suggestSelected = ref<string[]>([])
const suggestToken = ref({start:0,end:0})
const suggestPosition = ref({top:48,left:12,maxWidth:560})
const customTemplateDrafts = new WeakMap<object,string>()
let suggestTimer=0
let previewFlushTimer=0
let pendingPreviewPoints:PointStatus[]=[]
const channels = computed<any[]>(() => {
  if (!Array.isArray(store.value.mqttChannels)) store.value.mqttChannels = []
  return store.value.mqttChannels
})
const channel = computed(() => channels.value[activeChannel.value])
function mqttStatusKey(index:number) { return `manual-${index+1}` }
type TemplateSuggestion={key:string;label:string;kind:'variable'|'device'|'point';jsonKey?:string}
const templateVariables:TemplateSuggestion[]=[
  {key:'attribute',label:'按设备/点位多选',kind:'variable'},
  {key:'attributes',label:'当前批次全部点位键值对象',kind:'variable'},
  {key:'eventTime',label:'UTC 采集时间 yyyyMMddTHHmmssZ',kind:'variable'},
  {key:'devices',label:'按设备分组后的点位对象',kind:'variable'},
  {key:'gatewayKey',label:'网关标识',kind:'variable'},
  {key:'username',label:'MQTT Username',kind:'variable'},
  {key:'clientId',label:'MQTT Client ID',kind:'variable'},
  {key:'ts',label:'RFC3339 本地生成时间',kind:'variable'},
]
const sourceDevices=computed<any[]>(()=>[
  ...(store.value.devices||[]),
  ...(store.value.forwardDevices||[]),
  ...((store.value.edgeComputing?.groups||[]).map((group:any)=>({deviceKey:group.groupKey,name:group.name,points:group.points||[]}))),
])
const suggestions=ref<TemplateSuggestion[]>([])

function notify(text: string, failed = false) { message.value = failed ? '' : text; error.value = failed ? text : '' }
function markChannelsSaved() { savedChannels.value=JSON.stringify(channels.value); savedActivation.value=JSON.stringify({activation,activationFile:activationFile.value}); emit('dirty',false) }
function requestChannel(index:number) {
  if (index === activeChannel.value) return
  if (savedChannels.value && JSON.stringify(channels.value) !== savedChannels.value) { pendingChannel.value=index; mqttSwitchOpen.value=true; return }
  activeChannel.value=index
}
async function discardAndSwitch() {
  const target=pendingChannel.value
  mqttSwitchOpen.value=false
  await store.load()
  markChannelsSaved()
  activeChannel.value=Math.max(0,Math.min(target,channels.value.length-1))
}
async function saveAndSwitch() {
  const target=pendingChannel.value
  loading.value=true
  try { await store.save(); await store.load(); markChannelsSaved(); activeChannel.value=Math.max(0,Math.min(target,channels.value.length-1)); mqttSwitchOpen.value=false; notify('MQTT 通道已保存并重新连接') }
  catch (cause) { notify(cause instanceof Error ? cause.message : '保存失败', true) }
  finally { loading.value=false }
}
function addChannel() {
  const index = channels.value.length + 1
  channels.value.push({ name:`MQTT通道${index}`, enabled:true, broker:'tcp://192.168.1.100:1883', clientId:`${store.value.gatewayKey || 'gateway'}-${index}`, username:'', password:'', topicTemplate:'attributes', payloadMode:'flat', payloadTemplate:'', subscribeTopic:'', caFile:'', certFile:'', keyFile:'' })
  activeChannel.value = channels.value.length - 1
}
async function removeChannel(index: number) {
  const name = channels.value[index]?.name || String(index + 1)
  if (!await confirmAction({
    title: '删除 MQTT 通道',
    message: `确认删除 MQTT 通道“${name}”？`,
    detail: '删除后，该通道的连接与发布配置将一并移除。',
    confirmText: '确认删除',
    danger: true,
  })) return
  channels.value.splice(index, 1); activeChannel.value = Math.max(0, Math.min(activeChannel.value, channels.value.length - 1))
}
function brokerParts(value='') { const match=value.match(/^(mqtts?|ssl|tcp):\/\/([^:]+)(?::(\d+))?$/i); return { protocol:(match?.[1]||'mqtt').toLowerCase(), host:match?.[2]||value.replace(/^\w+:\/\//,'').split(':')[0]||'', port:Number(match?.[3]||((match?.[1]||'').match(/mqtts|ssl/i)?8883:1883)) } }
function brokerProtocol(value='') { return /^(mqtts|ssl)$/i.test(brokerParts(value).protocol) ? 'mqtts' : 'mqtt' }
function updateBroker(field:'protocol'|'host'|'port', value:string|number) { if(!channel.value)return; const parts=brokerParts(channel.value.broker); (parts as any)[field]=value; const scheme=parts.protocol==='mqtts'?'ssl':'tcp'; channel.value.broker=`${scheme}://${parts.host}:${parts.port}` }
function payloadModeValue(value='') { return ({gateway:'flat',device:'grouped',template:'custom'} as Record<string,string>)[value] || (['flat','grouped','custom'].includes(value) ? value : 'flat') }
const templateEditable=computed(()=>payloadModeValue(channel.value?.payloadMode)==='custom')
function updatePayloadMode(value:string) { if(!channel.value)return; const next=payloadModeValue(value),current=payloadModeValue(channel.value.payloadMode);if(current==='custom'&&channel.value.payloadTemplate)customTemplateDrafts.set(channel.value,channel.value.payloadTemplate);channel.value.payloadMode=next;channel.value.payloadTemplate=next==='custom'?(customTemplateDrafts.get(channel.value)||defaultMQTTPayloadTemplate()):'';suggestOpen.value=false }
function defaultMQTTPayloadTemplate() { return '{\n  "gatewayKey": "{gatewayKey}",\n  "ts": "{ts}",\n  "data": {attributes},\n  "devices": {devices}\n}' }
function groupedMQTTPayloadTemplate() { return '{\n  "gatewayKey": "{gatewayKey}",\n  "ts": "{ts}",\n  "devices": {devices}\n}' }
function huaweiMQTTPayloadTemplate() { return '{\n  "services": [\n    {\n      "service_id": "BasicData",\n      "properties": {attributes},\n      "eventTime": "{eventTime}"\n    }\n  ]\n}' }
function formatPayloadTemplate() {
  if(!channel.value||!templateEditable.value)return
  const rawMarkers=['attributes','devices']
  let text=channel.value.payloadTemplate || defaultMQTTPayloadTemplate()
  rawMarkers.forEach(key=>{text=text.split(`{${key}}`).join(`"__MQTT_RAW_${key}__"`)})
  const attributes:{marker:string,token:string}[]=[]
  text=text.replace(/\{attribute(?:\.[^{}\s",:\[\]]+){0,2}\}/g,(token:string)=>{const marker=`__MQTT_RAW_ATTRIBUTE_${attributes.length}__`;attributes.push({marker,token});return `"${marker}"`})
  try {
    let formatted=JSON.stringify(JSON.parse(text),null,2)
    rawMarkers.forEach(key=>{formatted=formatted.split(`"__MQTT_RAW_${key}__"`).join(`{${key}}`)})
    attributes.forEach(item=>{formatted=formatted.split(`"${item.marker}"`).join(item.token)})
    channel.value.payloadTemplate=formatted
    notify('JSON 模板已格式化对齐')
  } catch(cause) { notify(`JSON 模板格式化失败：${cause instanceof Error?cause.message:'格式错误'}`,true) }
}
function applyHuaweiPreset() { if(!channel.value)return; channel.value.topicTemplate='$oc/devices/{username}/sys/properties/report'; channel.value.payloadMode='custom'; channel.value.payloadTemplate=huaweiMQTTPayloadTemplate();customTemplateDrafts.set(channel.value,channel.value.payloadTemplate); notify('已套用华为云属性上报模板，请确认 service_id 和属性名与产品模型一致。') }
function payloadTemplateText(){const mode=payloadModeValue(channel.value?.payloadMode);return mode==='grouped'?groupedMQTTPayloadTemplate():mode==='custom'?(channel.value?.payloadTemplate||defaultMQTTPayloadTemplate()):'{attributes}'}
type PreviewPoint={deviceKey:string;metric:string;value:unknown}
function zeroPreviewValue(point:any){
  const dataType=String(point?.dataType||point?.type||'').toLowerCase()
  if(dataType==='bool'||dataType==='boolean')return false
  if(dataType==='string')return ''
  return 0
}
const templatePreviewPoints=computed<PreviewPoint[]>(()=>sourceDevices.value.flatMap(device=>(device.points||[]).filter((point:any)=>String(point.metric||'').trim()).map((point:any)=>({deviceKey:String(device.deviceKey||''),metric:String(point.metric),value:zeroPreviewValue(point)}))))
function renderPayload(points:PreviewPoint[],realtime=false){
  const attributes:Record<string,unknown>={},devices:Record<string,Record<string,unknown>>={}
  for(const point of points){attributes[point.metric]=point.value;(devices[point.deviceKey] ||= {})[point.metric]=point.value}
  const now=realtime?new Date():new Date(0),eventTime=now.toISOString().replace(/[-:]/g,'').replace(/\.\d{3}Z$/,'Z')
  let result=payloadTemplateText()
    .replace(/\{attributes\}/g,JSON.stringify(attributes))
    .replace(/\{devices\}/g,JSON.stringify(devices))
    .replace(/\{attribute(?:\.([^{}.\s]+))?(?:\.([^{}\s]+))?\}/g,(_token:string,deviceKey?:string,metric?:string)=>JSON.stringify(deviceKey&&metric?devices[deviceKey]?.[metric]??null:deviceKey?devices[deviceKey]||{}:attributes))
    .replace(/\{gatewayKey\}/g,store.value.gatewayKey||'gateway')
    .replace(/\{clientId\}/g,channel.value?.clientId||'')
    .replace(/\{username\}/g,channel.value?.username||'')
    .replace(/\{eventTime\}/g,eventTime)
    .replace(/\{ts\}/g,now.toISOString())
  try{result=JSON.stringify(JSON.parse(result),null,2)}catch{}
  return result
}
const payloadPreview=computed(()=>previewMode.value==='realtime'?renderPayload(pointStatuses.value,true):renderPayload(templatePreviewPoints.value))
function escapeHTML(value:string){return value.replace(/[&<>]/g,character=>({'&':'&amp;','<':'&lt;','>':'&gt;'}[character]||character))}
function highlightJSON(value:string,maxTokens=Number.POSITIVE_INFINITY){
  const tokenPattern=/\{[A-Za-z][^{}\s]*\}|"(?:\\.|[^"\\])*"(?=\s*:)|"(?:\\.|[^"\\])*"|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?|\b(?:true|false|null)\b/g
  let html='',cursor=0,count=0,match:RegExpExecArray|null
  while((match=tokenPattern.exec(value))){
    if(count>=maxTokens)return html+escapeHTML(value.slice(cursor))
    html+=escapeHTML(value.slice(cursor,match.index))
    const token=match[0]
    const rest=value.slice(match.index+token.length)
    const className=token.startsWith('{')?'json-token-variable':token.startsWith('"')?( /^\s*:/.test(rest)?'json-token-key':'json-token-string'):/^(true|false|null)$/.test(token)?'json-token-literal':'json-token-number'
    html+=`<span class="${className}">${escapeHTML(token)}</span>`
    cursor=match.index+token.length
    count++
  }
  return html+escapeHTML(value.slice(cursor))
}
const templateCodeHTML=computed(()=>highlightJSON(payloadTemplateText()))
const payloadPreviewHTML=computed(()=>highlightJSON(payloadPreview.value,700))
function updateSuggestPosition(textarea:HTMLTextAreaElement){
  const computedStyle=window.getComputedStyle(textarea),mirror=document.createElement('div'),marker=document.createElement('span')
  Object.assign(mirror.style,{position:'fixed',left:'-9999px',top:'0',visibility:'hidden',boxSizing:'border-box',whiteSpace:'pre-wrap',overflowWrap:'break-word',width:`${textarea.clientWidth}px`,padding:computedStyle.padding,border:computedStyle.border,font:computedStyle.font,lineHeight:computedStyle.lineHeight,letterSpacing:computedStyle.letterSpacing})
  mirror.textContent=textarea.value.slice(0,textarea.selectionStart||0);marker.textContent=textarea.value.slice(textarea.selectionStart||0,(textarea.selectionStart||0)+1)||' ';mirror.appendChild(marker);document.body.appendChild(mirror)
  const lineHeight=Number.parseFloat(computedStyle.lineHeight)||21,left=Math.max(8,Math.min(marker.offsetLeft-textarea.scrollLeft+14,textarea.clientWidth-300)),top=Math.max(8,marker.offsetTop-textarea.scrollTop+lineHeight+8)
  mirror.remove();suggestPosition.value={top,left,maxWidth:Math.max(280,textarea.clientWidth-left-8)}
}
const suggestStyle=computed(()=>({top:`${suggestPosition.value.top}px`,left:`${suggestPosition.value.left}px`,maxWidth:`${suggestPosition.value.maxWidth}px`}))
function syncTemplateScroll(event:Event){const textarea=event.target as HTMLTextAreaElement;if(templateHighlight.value){templateHighlight.value.scrollTop=textarea.scrollTop;templateHighlight.value.scrollLeft=textarea.scrollLeft}if(suggestOpen.value)updateSuggestPosition(textarea)}
function templateToken(textarea:HTMLTextAreaElement){const cursor=textarea.selectionStart||0,before=textarea.value.slice(0,cursor),match=before.match(/\{?(attribute(?:\.[^{}\s",:\[\]]*){0,2})$/i)||before.match(/\{?([A-Za-z][A-Za-z0-9_]*)$/);if(!match)return null;const end=textarea.value[cursor]==='}'?cursor+1:cursor;return{query:match[1],start:cursor-match[0].length,end}}
function suggestionItems(query:string):TemplateSuggestion[]{
  if(!query.toLowerCase().startsWith('attribute'))return templateVariables.filter(item=>item.key.toLowerCase().includes(query.toLowerCase()))
  const parts=query.split('.')
  if(parts.length===1)return templateVariables.filter(item=>item.key.toLowerCase().includes(query.toLowerCase()))
  if(parts.length===2){const term=(parts[1]||'').toLowerCase();return sourceDevices.value.filter(device=>String(device.deviceKey||'').toLowerCase().includes(term)||String(device.name||'').toLowerCase().includes(term)).map(device=>({key:`attribute.${device.deviceKey}`,label:`${device.name||device.deviceKey} 的全部点位`,kind:'device' as const,jsonKey:device.deviceKey}))}
  const device=sourceDevices.value.find(item=>String(item.deviceKey)===parts[1]),term=(parts[2]||'').toLowerCase()
  return (device?.points||[]).filter((point:any)=>String(point.metric||'').trim()).filter((point:any)=>String(point.metric).toLowerCase().includes(term)||String(point.name||'').toLowerCase().includes(term)).map((point:any)=>({key:`attribute.${device.deviceKey}.${point.metric}`,label:`${point.name||point.metric} · ${device.name||device.deviceKey}`,kind:'point' as const,jsonKey:point.metric}))
}
function refreshSuggestions(textarea:HTMLTextAreaElement){if(!templateEditable.value){suggestOpen.value=false;return}const token=templateToken(textarea);if(!token){suggestOpen.value=false;return}const items=suggestionItems(token.query);if(!items.length){suggestOpen.value=false;return}suggestions.value=items;suggestToken.value={start:token.start,end:token.end};suggestIndex.value=Math.min(suggestIndex.value,items.length-1);suggestSelected.value=suggestSelected.value.filter(key=>items.some(item=>item.key===key));suggestOpen.value=true;updateSuggestPosition(textarea)}
async function handleTemplateInput(event:Event){if(!templateEditable.value)return;const textarea=event.target as HTMLTextAreaElement,input=event as InputEvent;channel.value.payloadTemplate=textarea.value;let cursor=textarea.selectionStart||0;if(input.inputType?.startsWith('insert')&&textarea.value.slice(0,cursor).match(/(?:^|[\s{",:])attribute$/i)){channel.value.payloadTemplate=textarea.value.slice(0,cursor)+'.'+textarea.value.slice(cursor);cursor++;await nextTick();textarea.setSelectionRange(cursor,cursor)}refreshSuggestions(textarea)}
function toggleSuggestion(key:string){const index=suggestSelected.value.indexOf(key);if(index>=0)suggestSelected.value.splice(index,1);else suggestSelected.value.push(key)}
function suggestionInsertion(items:TemplateSuggestion[]){if(items.length===1){const item=items[0];if(item.kind==='variable'&&item.key==='attribute'){const value='{attribute.';return{value,cursor:value.length,continued:true}}const value=`{${item.key}}`;return{value,cursor:item.kind==='device'?value.length-1:value.length,continued:false}}const pairs=items.map(item=>`  "${item.jsonKey||item.key.split('.').pop()}": {${item.key}}`),value=`{\n${pairs.join(',\n')}\n}`;return{value,cursor:value.length,continued:false}}
async function insertSuggestions(){const items=suggestSelected.value.length?suggestions.value.filter(item=>suggestSelected.value.includes(item.key)):[suggestions.value[suggestIndex.value]].filter(Boolean);if(!items.length||!channel.value)return;const insertion=suggestionInsertion(items),text=channel.value.payloadTemplate||'';channel.value.payloadTemplate=text.slice(0,suggestToken.value.start)+insertion.value+text.slice(suggestToken.value.end);const cursor=suggestToken.value.start+insertion.cursor;suggestOpen.value=false;suggestSelected.value=[];await nextTick();if(!templateEditor.value)return;templateEditor.value.focus();templateEditor.value.setSelectionRange(cursor,cursor);if(insertion.continued)refreshSuggestions(templateEditor.value)}
function handleTemplateKey(event:KeyboardEvent){if(suggestOpen.value&&['ArrowDown','ArrowUp'].includes(event.key)){event.preventDefault();const delta=event.key==='ArrowDown'?1:-1;suggestIndex.value=(suggestIndex.value+delta+suggestions.value.length)%suggestions.value.length;return}if(suggestOpen.value&&event.key===' '){event.preventDefault();toggleSuggestion(suggestions.value[suggestIndex.value].key);return}if(suggestOpen.value&&['Enter','Tab'].includes(event.key)){event.preventDefault();void insertSuggestions();return}if(event.key==='Escape'){suggestOpen.value=false;return}if(event.key==='Tab'){event.preventDefault();const textarea=event.target as HTMLTextAreaElement,start=textarea.selectionStart||0,end=textarea.selectionEnd||0;channel.value.payloadTemplate=textarea.value.slice(0,start)+'  '+textarea.value.slice(end);nextTick(()=>textarea.setSelectionRange(start+2,start+2))}}
function handleTemplateKeyUp(event:KeyboardEvent){if(['ArrowLeft','ArrowRight','Home','End'].includes(event.key))refreshSuggestions(event.target as HTMLTextAreaElement)}
function hideSuggestionsSoon(){window.clearTimeout(suggestTimer);suggestTimer=window.setTimeout(()=>suggestOpen.value=false,160)}
function queuePreviewPoints(points:PointStatus[]){
  pendingPreviewPoints=points
  if(!pointStatuses.value.length){pointStatuses.value=points;return}
  if(previewFlushTimer)return
  previewFlushTimer=window.setTimeout(()=>{pointStatuses.value=pendingPreviewPoints;previewFlushTimer=0},1000)
}
async function importActivation(event:Event) { const input=event.target as HTMLInputElement,file=input.files?.[0]; if(!file)return; try{const parsed=JSON.parse(await file.text());Object.assign(activation,parsed);activationFile.value=file.name||activationFile.value;notify('已导入激活文件，请检查后点击“保存激活配置”。')}catch(cause){notify(cause instanceof Error?cause.message:'导入激活文件失败',true)}finally{input.value=''} }
async function load() {
  loading.value = true
  try {
    await store.load()
    const data = await api<{content:string}>('/api/activation')
    const parsed = JSON.parse(data.content || '{}')
    Object.assign(activation, { enabled:store.value.activation?.enabled !== false && parsed.enabled !== false, hardwareId:parsed.hardwareId || '', sn:parsed.sn || '', broker:parsed.broker || '', deviceSecret:parsed.deviceSecret || '' })
    activationFile.value = store.value.activation?.file || 'activation.local.json'
    activeChannel.value = Math.max(0, Math.min(activeChannel.value, channels.value.length - 1))
    markChannelsSaved()
    notify('')
  } catch (cause) { notify(cause instanceof Error ? cause.message : '云连接读取失败', true) }
  finally { loading.value = false }
}
async function saveChannels() {
  loading.value = true
  try { await store.save(); notify('MQTT 通道已保存并重新连接'); await load() }
  catch (cause) { notify(cause instanceof Error ? cause.message : '保存失败', true) }
  finally { loading.value = false }
}
async function testPublish() {
  loading.value = true
  try { const result = await api<{topic:string}>('/api/mqtt/test-publish', { method:'POST' }); notify(`测试发布成功：${result.topic}`) }
  catch (cause) { notify(cause instanceof Error ? cause.message : '测试发布失败', true) }
  finally { loading.value = false }
}
async function saveActivation() {
  loading.value = true
  try {
    const data = await api<{file:string}>('/api/activation', { method:'PUT', headers:{'Content-Type':'application/json'}, body:JSON.stringify({ file:activationFile.value, content:JSON.stringify(activation) }) })
    activationFile.value = data.file || activationFile.value
    notify(activation.enabled ? '微控云配置已保存，正在重新连接' : '微控云已停用')
    window.setTimeout(load, 1200)
  } catch (cause) { notify(cause instanceof Error ? cause.message : '保存失败', true) }
  finally { loading.value = false }
}
async function saveBeforeLeave() {
  loading.value=true
  try {
    await store.save()
    await api('/api/activation',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({file:activationFile.value,content:JSON.stringify(activation)})})
    await load()
    return true
  } catch (cause) { notify(cause instanceof Error?cause.message:'保存失败',true); return false }
  finally { loading.value=false }
}
watch(()=>[channels.value,activation,activationFile.value],()=>{if(savedChannels.value)emit('dirty',JSON.stringify(channels.value)!==savedChannels.value||JSON.stringify({activation,activationFile:activationFile.value})!==savedActivation.value)},{deep:true})
watch(()=>sharedStatus?.value?.points,points=>{if(Array.isArray(points))queuePreviewPoints(points)},{immediate:true})
defineExpose({saveBeforeLeave})
onMounted(load)
onBeforeUnmount(()=>{window.clearTimeout(suggestTimer);window.clearTimeout(previewFlushTimer);pendingPreviewPoints=[]})
</script>

<template><section id="panel-cloud"><p v-if="message" class="notice good">{{message}}</p><p v-if="error" class="notice bad">{{error}}</p><section class="card"><div class="section-title"><div><h2>云平台连接</h2><p class="meta">配置 MQTT 云平台连接。停用后网关只做本地采集和展示，不会上报数据。</p></div><div><button @click="load">重新加载</button><button class="primary" :disabled="loading" @click="saveChannels">保存配置</button></div></div>
  <div class="mqtt-panel"><div class="mqtt-head"><div><h3>手动 MQTT 连接</h3><p class="meta">可创建多个手动 MQTT 通道，不同通道连接不同平台并上报同一批采集数据。</p></div><button type="button" @click="addChannel">新增通道</button></div>
    <div v-if="channels.length" class="mqtt-channel-tabs"><button v-for="(item,index) in channels" :key="index" type="button" class="mqtt-channel-tab" :class="{active:index===activeChannel}" @click="requestChannel(index)">{{item.name||`手动 MQTT ${index+1}`}}<i class="mqtt-tab-status" :class="{online:item.enabled!==false&&status?.mqttChannels?.[mqttStatusKey(index)]?.connected,offline:item.enabled!==false&&!status?.mqttChannels?.[mqttStatusKey(index)]?.connected}"/></button></div>
    <div v-if="channel" class="mqtt-channel-row">
      <div class="mqtt-basic-grid"><label>名称<input v-model.trim="channel.name"/></label><label>状态<select v-model="channel.enabled"><option :value="true">启用</option><option :value="false">停用</option></select></label><div class="mqtt-basic-spacer"/><div class="cloud-conn-status"><span>连接状态</span><div class="cloud-conn-pill"><i class="cloud-conn-dot" :class="status?.mqttChannels?.[mqttStatusKey(activeChannel)]?.connected?'online':'offline'"/>{{channel.enabled===false?'未启用':status?.mqttChannels?.[mqttStatusKey(activeChannel)]?.connected?'连接成功':'未连接'}}<svg v-if="channel.enabled!==false&&status?.mqttChannels?.[mqttStatusKey(activeChannel)]?.connected" class="cloud-conn-ecg" viewBox="0 0 38 14" aria-label="连接心跳"><path d="M1 8h8l2-4 3 8 4-11 4 10 3-3h12"/></svg></div></div><div class="point-actions"><button class="danger" @click="removeChannel(activeChannel)">删除</button></div></div>
      <div class="mqtt-broker-grid"><label>协议<select :value="brokerProtocol(channel.broker)" @change="updateBroker('protocol',($event.target as HTMLSelectElement).value)"><option value="mqtt">MQTT</option><option value="mqtts">MQTTS</option></select></label><label>Broker 地址<input :value="brokerParts(channel.broker).host" placeholder="例如 127.0.0.1" @input="updateBroker('host',($event.target as HTMLInputElement).value)"/></label><label>端口<input :value="brokerParts(channel.broker).port" type="number" @input="updateBroker('port',Number(($event.target as HTMLInputElement).value))"/></label></div>
      <div class="mqtt-auth-grid"><label>Client ID<input v-model.trim="channel.clientId"/></label><label>Username<input v-model.trim="channel.username"/></label><label>Password<input v-model="channel.password" type="password" autocomplete="new-password"/></label></div>
      <div class="mqtt-subscribe-grid"><label>订阅主题<input v-model.trim="channel.subscribeTopic" placeholder="例如 attributes/set"/></label></div>
      <div class="mqtt-publish-grid"><label>发布主题<input v-model.trim="channel.topicTemplate" placeholder="attributes 或 gateway/{gatewayKey}/telemetry"/></label><label>JSON 格式<select :value="payloadModeValue(channel.payloadMode)" @change="updatePayloadMode(($event.target as HTMLSelectElement).value)"><option value="flat">扁平键值</option><option value="grouped">按设备分组</option><option value="custom">自定义 JSON</option></select></label></div>
      <div class="mqtt-template-row"><div class="mqtt-template-head"><span>JSON 模板 <small v-if="!templateEditable">（当前格式由系统生成）</small></span><div class="mqtt-template-tools"><button type="button" :disabled="!templateEditable" @click="formatPayloadTemplate">格式化对齐</button><button type="button" @click="applyHuaweiPreset">套用华为云属性上报</button></div></div><div class="mqtt-template-layout"><div class="mqtt-editor-wrap"><div class="mqtt-code-shell" :class="{readonly:!templateEditable}"><pre ref="templateHighlight" class="mqtt-code-highlight" aria-hidden="true" v-html="templateCodeHTML"/><textarea ref="templateEditor" :value="payloadTemplateText()" class="mqtt-code-editor" :readonly="!templateEditable" spellcheck="false" autocomplete="off" @input="handleTemplateInput" @scroll="syncTemplateScroll" @focus="refreshSuggestions($event.target as HTMLTextAreaElement)" @click="refreshSuggestions($event.target as HTMLTextAreaElement)" @keyup="handleTemplateKeyUp" @keydown="handleTemplateKey" @blur="hideSuggestionsSoon"/></div><div v-if="suggestOpen" class="mqtt-template-suggest" :style="suggestStyle"><div class="mqtt-suggest-head"><span>智能提示</span><small>↑↓ 选择 · 空格多选 · Enter 插入</small></div><button v-for="(item,index) in suggestions" :key="item.key" type="button" :class="{active:index===suggestIndex,selected:suggestSelected.includes(item.key)}" @mousedown.prevent="toggleSuggestion(item.key)"><i>{{suggestSelected.includes(item.key)?'✓':''}}</i><code>{ {{item.key}} }</code><span>{{item.label}}</span></button><div class="mqtt-suggest-actions"><span>已选择 {{suggestSelected.length}} 项</span><button type="button" @mousedown.prevent="insertSuggestions">插入所选</button></div></div><p v-if="templateEditable" class="mqtt-template-hint">输入 attribute 后先选择设备；如需继续选择具体点位，在设备占位符的右花括号前输入“.”。支持空格多选、方向键浏览、回车插入。</p><p v-else class="mqtt-template-hint">扁平键值和按设备分组使用系统固定结构；选择“自定义 JSON”后可编辑模板。</p></div><div class="mqtt-template-preview"><div class="mqtt-template-preview-title"><span>{{previewMode==='template'?'零值结构预览':'实时上报预览'}}</span><div class="mqtt-preview-switch"><button type="button" :class="{active:previewMode==='template'}" @click="previewMode='template'">模板预览</button><button type="button" :class="{active:previewMode==='realtime'}" @click="previewMode='realtime'">实时点位</button></div></div><pre v-html="payloadPreviewHTML"/></div></div></div>
    </div><div v-else class="mqtt-channel-empty">暂无手动 MQTT 通道，点击“新增通道”创建。</div>
  </div>
  <div class="mqtt-panel activation-panel"><div class="mqtt-head activation-head"><div><h3>微控云连接</h3><p class="meta">使用云平台下发的激活文件连接，通常用于设备预注册和自动绑定。</p></div><label class="mqtt-switch">微控云状态<select v-model="activation.enabled"><option :value="true">启用</option><option :value="false">停用</option></select></label><div class="cloud-conn-status"><span>连接状态</span><div class="cloud-conn-pill"><i class="cloud-conn-dot" :class="activation.enabled&&status?.mqttChannels?.activation?.connected?'online':'offline'"/>{{activation.enabled?(status?.mqttChannels?.activation?.connected?'连接成功':'未连接'):'未启用'}}<svg v-if="activation.enabled&&status?.mqttChannels?.activation?.connected" class="cloud-conn-ecg" viewBox="0 0 38 14" aria-label="连接心跳"><path d="M1 8h8l2-4 3 8 4-11 4 10 3-3h12"/></svg></div></div></div><div class="activation-grid"><label>激活文件<input v-model.trim="activationFile"/></label><label>HardwareID<input v-model.trim="activation.hardwareId"/></label><label>SN<input v-model.trim="activation.sn"/></label><label>Broker<input v-model.trim="activation.broker"/></label><label>DeviceSecret<input v-model="activation.deviceSecret" type="password" autocomplete="new-password"/></label></div><div class="point-actions activation-actions"><input ref="activationImport" class="hidden-file" type="file" accept=".json,application/json" @change="importActivation"/><button @click="activationImport?.click()">导入激活文件</button><button @click="load">读取激活文件</button><button class="primary" :disabled="loading" @click="saveActivation">保存激活配置</button></div></div>
</section><div v-if="mqttSwitchOpen" class="mqtt-switch-modal"><div class="dialog-panel"><h3>当前 MQTT 通道尚未保存</h3><p>切换通道前请选择如何处理当前修改。</p><div class="dialog-actions"><button @click="mqttSwitchOpen=false">继续编辑</button><button class="danger" @click="discardAndSwitch">放弃修改并切换</button><button class="primary" :disabled="loading" @click="saveAndSwitch">保存修改并切换</button></div></div></div></section></template>
