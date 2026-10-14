<template>
    <div class="go-switchFun">
        <n-switch aria-label="点位开关" :value="dataV.dataset===true" :disabled="!canControl||busy||uncertain||!valueMapping||valueTypeMismatch||dataV.dataset===undefined" :loading="busy" :unchecked-value="false" :checked-value="true" @update:value="changeControl" />
        <button v-if="runtime&&props.chartConfig.option.controlEnabled&&!busy&&(uncertain||!canControl)" class="recovery-button" :aria-label="uncertain?'查询回执':'检查连接与权限'" @click.stop="uncertain?refreshReceipt():refreshCapability()">↻</button>
    </div>
</template>

<script setup lang="ts">
import { PropType, ref, toRefs, watch, computed, onBeforeUnmount } from 'vue'
import { CreateComponentType } from '@/packages/index.d'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { useChartDataFetch } from '@/hooks'
import { isPreview } from '@/utils/router'
import { request } from '@/utils/request'
import { goDialog } from '@/utils/plugin'
import { switchBoundPoint, switchWriteValue, switchValueMapping, commandApiData } from '@/utils/switchControl'

const props = defineProps({
    chartConfig: {
        type: Object as PropType<CreateComponentType>,
        required: true
    }
})

// 解构容器宽高和样式属性
const { w, h } = toRefs(props.chartConfig.attr)
const { dataset, kaiColor, guanColor, valueType } = toRefs(props.chartConfig.option)
const runtime=isPreview(),busy=ref(false),uncertain=ref(false),canControl=ref(false),controlMessage=ref(runtime?'只读显示':'编辑状态不下发')
const commandId=ref(''),point=computed(()=>switchBoundPoint(props.chartConfig.request))
const rawValue=ref<unknown>(props.chartConfig.option.dataset),reportedType=ref('')
const declaredType=computed(()=>{
    const raw:any=props.chartConfig.request?.requestParams?.Params
    const binding=(Array.isArray(raw)?raw:Object.values(raw||{})).find((p:any)=>p?.value1===point.value?.value1&&p?.value2===point.value?.value2) as any
    return reportedType.value||binding?.TypeOptions?.find((m:any)=>m.value===point.value?.value2)?.dataType||''
})
const valueMapping=computed(()=>switchValueMapping(props.chartConfig.option,rawValue.value,declaredType.value))
const valueTypeMismatch=computed(()=>props.chartConfig.option.valueMode==='manual'&&!!declaredType.value&&!!valueMapping.value&&valueMapping.value.type!==String(declaredType.value).toUpperCase().replace(/^BOOL$/,'BOOLEAN'))
let disposed=false,generation=0,commandDevice=''
const pendingKey=`wk_SwitchPending:${props.chartConfig.id}`,pendingRequestId=ref('')
if(runtime){try{const pending=JSON.parse(sessionStorage.getItem(pendingKey)||'null');if(pending?.deviceId&&pending?.requestId){commandDevice=pending.deviceId;commandId.value=pending.commandId||'';pendingRequestId.value=pending.requestId;uncertain.value=true;controlMessage.value='有未确认的下发，请查询回执后再操作'}}catch{}}
const persistPending=()=>sessionStorage.setItem(pendingKey,JSON.stringify({deviceId:commandDevice,commandId:commandId.value,requestId:pendingRequestId.value}))
const errorText=(error:any)=>error?.response?.data?.message||error?.message||'控制请求失败'
const notifyControl=(type:'success'|'warning',message:string)=>{if(!disposed&&typeof window!=='undefined')window['$message']?.[type](message,{duration:3500})}
const endpoint=(id:string)=>`/devices/${encodeURIComponent(id)}/commands`
async function capabilities(){
    const p=point.value
    if(!isPreview()||!props.chartConfig.option.controlEnabled||!p){canControl.value=false;controlMessage.value=!isPreview()?'编辑状态不下发':!props.chartConfig.option.controlEnabled?'只读显示':'请绑定一个实时点位';return null}
    const caps=commandApiData(await request({url:endpoint(p.value1)+'/capabilities',method:'GET'}))
    const writable=caps?.points?.find((item:any)=>item.identifier===p.value2)
    reportedType.value=writable?.dataType||''
    canControl.value=!!caps?.available&&!!writable
    if(!canControl.value)controlMessage.value=caps?.reason||'该点位不可写'
    return canControl.value?writable:null
}
watch(()=>JSON.stringify([point.value,props.chartConfig.option.controlEnabled]),async()=>{
    const current=++generation;canControl.value=false
    if(busy.value||uncertain.value)return
    try{await capabilities();if(!disposed&&current===generation&&canControl.value)controlMessage.value='可控制 · 点击后确认下发'}catch(error){if(current===generation)controlMessage.value=errorText(error)}
},{immediate:true})
async function checkReceipt(){
    if(!pendingRequestId.value||!commandDevice||disposed)return
    const rows=commandApiData(await request({url:endpoint(commandDevice),method:'GET'}))
    const task=Array.isArray(rows)?rows.find((item:any)=>item.id===commandId.value||item.requestId===pendingRequestId.value):null
    if(!task){uncertain.value=true;controlMessage.value='回执暂不可确认，请检查实际状态，勿重复下发';return}
    commandId.value=task.id;persistPending()
    if(task.status==='SUCCEEDED'){uncertain.value=false;sessionStorage.removeItem(pendingKey);controlMessage.value='网关已执行，状态以实时上报为准';notifyControl('success',controlMessage.value);return 'done'}
    if(task.status==='FAILED'){uncertain.value=false;sessionStorage.removeItem(pendingKey);controlMessage.value=task.message||'网关执行失败';notifyControl('warning',controlMessage.value);return 'done'}
    if(task.status==='TIMEOUT'){uncertain.value=true;controlMessage.value='结果未知，请检查实际状态，勿重复下发';notifyControl('warning',controlMessage.value);return 'done'}
    controlMessage.value='已发送，等待网关回执';return 'waiting'
}
async function refreshReceipt(){try{await checkReceipt();if(!uncertain.value)await capabilities()}catch{controlMessage.value='回执查询失败，请检查实际状态，勿重复下发';notifyControl('warning',controlMessage.value)}}
async function refreshCapability(){try{if(await capabilities()){controlMessage.value='可控制 · 点击后确认下发';notifyControl('success',controlMessage.value)}else notifyControl('warning',controlMessage.value)}catch(error){controlMessage.value=errorText(error);notifyControl('warning',controlMessage.value)}}
async function changeControl(value:any){
    if(!isPreview()||!props.chartConfig.option.controlEnabled||busy.value||uncertain.value)return
    busy.value=true;const current=generation
    try{
        const writable=await capabilities(),p=point.value
        if(!writable||!p||current!==generation)return
        if(typeof value!=='boolean'||dataV.value.dataset===undefined)return
        const writeValue=switchWriteValue(value,writable.dataType,props.chartConfig.option,rawValue.value)
        const confirmed=await new Promise<boolean>((resolve)=>goDialog({
            title:'确认点位下发',
            message:`点位：${p.value2}\n目标值：${JSON.stringify(writeValue)}\n此操作将写入设备，请确认后继续。`,
            positiveText:'确认下发',negativeText:'取消',
            onPositiveCallback:()=>resolve(true),onNegativeCallback:()=>resolve(false),
            onClose:()=>resolve(false),onAfterLeave:()=>resolve(false)
        }))
        if(!confirmed)return
        if(disposed||!isPreview()||current!==generation||!props.chartConfig.option.controlEnabled)return
        const requestId=crypto.randomUUID();commandId.value='';commandDevice=p.value1
        // 一次人工确认只发送一次POST，网络或认证恢复都不自动重发。
        uncertain.value=true;controlMessage.value='正在下发，请勿重复操作'
        pendingRequestId.value=requestId;persistPending()
        const task=commandApiData(await request({url:endpoint(p.value1),method:'POST',data:{requestId,metric:p.value2,value:writeValue}}))
        commandId.value=task.id;persistPending()
        for(let i=0;i<18&&!disposed;i++){
            if(await checkReceipt()==='done')break
            await new Promise(resolve=>setTimeout(resolve,2000))
        }
        if(!disposed&&uncertain.value)controlMessage.value='结果尚未确认，请检查实际状态，勿重复下发'
    }catch(error:any){
        const status=error?.status||error?.response?.status
        if(!commandId.value&&(status===400||status===403||status===404)){uncertain.value=false;sessionStorage.removeItem(pendingKey)}
        controlMessage.value=uncertain.value?'发送结果未知，请检查实际状态，勿重复下发':errorText(error)
        notifyControl('warning',controlMessage.value)
    }finally{busy.value=false}
}
onBeforeUnmount(()=>{disposed=true;generation++})

const dataV=ref<{dataset:boolean|undefined}>({dataset:undefined})
watch([rawValue,valueMapping],()=>{
    const mapping=valueMapping.value
    dataV.value.dataset=mapping&&rawValue.value===mapping.on?true:mapping&&rawValue.value===mapping.off?false:undefined
},{immediate:true})
const updateNumber=(newData:any)=>{rawValue.value=newData}
watch(()=>props.chartConfig.option.dataset,updateNumber)
watch(()=>JSON.stringify(point.value),()=>{reportedType.value='';rawValue.value=undefined})

useChartDataFetch(props.chartConfig, useChartEditStore,updateNumber)
</script>

<style lang="scss" scoped>
.go-switchFun{position:relative;width:100%;height:100%}
.recovery-button{position:absolute;left:calc(100% + 4px);top:50%;transform:translateY(-50%);width:22px;height:22px;padding:0;border:0;border-radius:4px;background:transparent;color:#9aa8b7;cursor:pointer}
.recovery-button:hover,.recovery-button:focus-visible{color:#63d6c0;outline:1px solid currentColor}
:deep(.n-switch){
    display: block;
    height: 100%;
}
:deep(.n-switch .n-switch__rail){
    background-color: v-bind(kaiColor);
    height: 100%;
}
:deep(.n-switch.n-switch--active .n-switch__rail){
    background-color: v-bind(guanColor);
}
:deep(.n-switch .n-switch__rail .n-switch__button){
    height: inherit;
    width: 40%;
    border-radius: 50%;
    max-width: none;
    left: 0;
    top: 0;
}
:deep(.n-switch.n-switch--active .n-switch__rail .n-switch__button){
    left: 60%;
}
:deep(.n-switch.n-switch--round .n-switch__rail) {
    border-radius: 100px;
}
:deep(.n-switch:not(.n-switch--disabled):not(.n-switch--icon).n-switch--rubber-band .n-switch__rail:active .n-switch__button){
    max-width: none;
}
</style>
