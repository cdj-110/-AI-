<script setup lang="ts">
import {inject,onBeforeUnmount,onMounted,ref,watch} from 'vue'
import ConfirmationDialog from './ConfirmationDialog.vue'
import {confirmAction} from '../confirm'
import {defaultAlarmFile} from '../alarmBundledAudio.ts'
import {alarmPresentationKey} from '../alarmPresentation.ts'
import {useAlarmSoundDraft} from '../composables/useAlarmSoundDraft.ts'
import {pageLeaveGuardKey,type RegisterPageLeaveGuard} from '../composables/usePageDraftGuard.ts'
import {alarmLevels,alarmSoundTones,alarmLevelSettings,normalizeAlarmSoundSettings,type AlarmLevelSettings} from '../alarmSound.ts'
const audio=inject(alarmPresentationKey)
const editor=useAlarmSoundDraft(()=>audio?.settings.value||normalizeAlarmSoundSettings({}),()=>audio?.scope()||'',value=>audio?.configure(value)===true)
const {draft,dirty,pending,busy,notice,conflict,uploading,uploadError}=editor
watch(()=>draft.value.enabled,enabled=>{if(!enabled)audio?.stopPreview()},{flush:'sync'})
function setLevel(level:string,key:keyof AlarmLevelSettings,value:unknown){editor.edit({...draft.value,levels:{...draft.value.levels,[level]:{...alarmLevelSettings(draft.value,level),[key]:value}}})}
function resetLevel(level:string){editor.clearAudio(level);editor.edit({...draft.value,levels:{...draft.value.levels,[level]:alarmLevelSettings(normalizeAlarmSoundSettings({}),level)}})}
function upload(level:string,event:Event){const input=event.target as HTMLInputElement,file=input.files?.[0];input.value='';if(file)void editor.uploadAudio(level,file)}
function save(){audio?.stopPreview();return editor.save()}
function useProvided(){for(const level of alarmLevels){editor.clearAudio(level.key);setLevel(level.key,'tone',defaultAlarmFile(level.key))}}
const leaveOpen=ref(false)
let resolveLeave:((value:boolean)=>void)|undefined
let leavePromise:Promise<boolean>|undefined
function requestLeave():Promise<boolean>{
 if(!pending.value)return Promise.resolve(true)
 if(leavePromise)return leavePromise
 leaveOpen.value=true
 leavePromise=new Promise(resolve=>{resolveLeave=resolve})
 return leavePromise
}
function finishLeave(value:boolean){leaveOpen.value=false;resolveLeave?.(value);resolveLeave=undefined;leavePromise=undefined}
function saveAndLeave(){if(save())finishLeave(true)}
function discardAndLeave(){editor.reset();audio?.stopPreview();finishLeave(true)}
async function discard(){if(await confirmAction({title:'放弃声音与颜色修改？',message:'本次修改和未保存的上传音频将被放弃，继续使用已保存设置。',confirmText:'放弃修改',cancelText:'继续编辑',danger:true})){editor.reset();audio?.stopPreview()}}
const register=inject<RegisterPageLeaveGuard|undefined>(pageLeaveGuardKey,undefined)
const unregister=register?.(requestLeave)
function beforeUnload(event:BeforeUnloadEvent){if(pending.value){event.preventDefault();event.returnValue=''}}
onMounted(()=>window.addEventListener('beforeunload',beforeUnload))
onBeforeUnmount(()=>{finishLeave(false);unregister?.();window.removeEventListener('beforeunload',beforeUnload);audio?.stopPreview();editor.dispose()})
defineExpose({requestLeave})
</script>
<template>
 <section v-if="audio" class="alarm-sound-settings" aria-label="告警声音设置">
  <div class="sound-heading"><div><h3>分级声音与颜色</h3><p>修改后点击“保存设置”生效；保存在当前浏览器和账号，并同步到告警标签页。</p></div><label class="sound-master-control"><span>声音总开关</span><span class="sound-master-switch"><input type="checkbox" role="switch" aria-label="告警声音开关" :aria-checked="draft.enabled" :checked="draft.enabled" @change="editor.edit({...draft,enabled:($event.target as HTMLInputElement).checked})"/><i aria-hidden="true"/></span><span class="sound-master-state" :class="{on:draft.enabled}">{{draft.enabled?'已开启':'已关闭'}}</span></label></div>
  <div class="sound-save-bar"><span role="status">{{busy?'音频正在校验…':dirty?'有未保存的修改':notice||'设置已保存'}}</span><div><button type="button" :disabled="!pending" @click="discard">放弃修改</button><button type="button" class="primary" :disabled="!dirty||busy||conflict" @click="save">保存设置</button></div></div>

  <p v-if="conflict" class="notice" role="alert">其他标签页已更新设置。为避免覆盖，请放弃本页修改后重新编辑。</p>
  <p v-if="notice&&dirty" class="notice" role="alert">{{notice}}</p>
  <p v-if="!draft.enabled" class="sound-off-notice" role="status">{{audio.settings.value.enabled?'已选择关闭，请点击“保存设置”停止告警声音。':'告警声音已关闭，开启总开关后可编辑以下配置。'}}关闭声音不会停止告警记录和画面提示。</p>
  <fieldset class="sound-config-body" :class="{'is-disabled':!draft.enabled}" :disabled="!draft.enabled" :aria-disabled="!draft.enabled" aria-label="告警声音配置区">
  <div class="sound-footer"><button type="button" class="stop-preview" @click="useProvided">一键使用分级音频</button><span>提示：Buzzer4 · 警告：Buzzer1 · 严重：Alarm2；Siren1 为备用警笛。选择后需保存。</span></div>
  <div class="level-settings" role="group" aria-label="分级声音与颜色配置">
   <div class="level-settings-head" aria-hidden="true"><span>声音 / 等级</span><span>告警声音</span><span>播放方式</span><span>音量</span><span>告警颜色</span><span>操作</span></div>
   <section v-for="level in alarmLevels" :key="level.key" class="level-settings-row" :aria-label="level.name+'等级设置'">
    <label class="level-toggle"><input type="checkbox" :aria-label="level.name+'声音开关'" :checked="alarmLevelSettings(draft,level.key).enabled" @change="setLevel(level.key,'enabled',($event.target as HTMLInputElement).checked)"/><strong :style="{color:alarmLevelSettings(draft,level.key).color}">{{level.name}}</strong></label>
    <div class="tone-cell"><label><span class="mobile-label">告警声音</span><select :aria-label="level.name+'提示音'" :value="alarmLevelSettings(draft,level.key).tone" @change="setLevel(level.key,'tone',($event.target as HTMLSelectElement).value)"><option v-for="tone in alarmSoundTones" :key="tone.key" :value="tone.key">{{tone.name}}</option><option v-if="alarmLevelSettings(draft,level.key).customAudio" value="custom">自定义：{{alarmLevelSettings(draft,level.key).customAudio?.name}}</option></select></label><div class="upload-line"><label class="upload-button" :class="{busy:uploading[level.key]}">{{uploading[level.key]?'正在校验…':alarmLevelSettings(draft,level.key).customAudio?'替换音频':'上传音频'}}<input type="file" accept=".mp3,.wav,.ogg,audio/mpeg,audio/wav,audio/ogg" :aria-label="level.name+'上传音频'" :disabled="uploading[level.key]" @change="upload(level.key,$event)"/></label><span v-if="alarmLevelSettings(draft,level.key).customAudio" class="file-name" :title="alarmLevelSettings(draft,level.key).customAudio?.name">{{alarmLevelSettings(draft,level.key).customAudio?.name}} · {{alarmLevelSettings(draft,level.key).customAudio?.duration.toFixed(1)}} 秒</span><button v-if="alarmLevelSettings(draft,level.key).customAudio" type="button" class="text-button" :aria-label="level.name+'移除音频'" @click="editor.clearAudio(level.key)">移除</button></div><p v-if="uploadError[level.key]" role="alert" class="upload-error">{{uploadError[level.key]}}</p></div>
    <label><span class="mobile-label">播放方式</span><select :aria-label="level.name+'播放方式'" :value="alarmLevelSettings(draft,level.key).mode" @change="setLevel(level.key,'mode',($event.target as HTMLSelectElement).value)"><option value="loop">持续循环</option><option value="once">新告警播放一次</option></select></label>
    <label class="volume-cell"><span class="mobile-label">音量</span><input :aria-label="level.name+'告警音量'" type="range" min="0" max="100" step="5" :value="alarmLevelSettings(draft,level.key).volume" @input="setLevel(level.key,'volume',Number(($event.target as HTMLInputElement).value))"/><output>{{alarmLevelSettings(draft,level.key).volume}}%</output></label>
    <label class="color-control"><span class="mobile-label">告警颜色</span><input type="color" :aria-label="level.name+'告警颜色'" :value="alarmLevelSettings(draft,level.key).color" @input="setLevel(level.key,'color',($event.target as HTMLInputElement).value)"/><code>{{alarmLevelSettings(draft,level.key).color.toUpperCase()}}</code></label>
    <div class="row-actions"><button type="button" class="preview" :aria-label="level.name+'试听'" @click="audio.preview(level.key,draft)">试听</button><button type="button" class="text-button" :aria-label="level.name+'恢复默认'" @click="resetLevel(level.key)">恢复默认</button></div>
   </section>
   <p class="upload-hint">支持 MP3 / WAV / OGG，单个不超过 2 MB、30 秒；上传后加入待保存设置。音频仅保存在此浏览器，清理站点数据后需重新上传。</p>
  </div>
  <div class="sound-footer"><button type="button" class="stop-preview" @click="audio.stopPreview">停止试听</button><span>多个告警优先播放最高未静音等级，声音不会叠加。</span></div>
  </fieldset>
  <p v-if="audio.blocked.value&&draft.enabled" role="status" class="notice">浏览器尚未允许播放声音，请点击“试听”启用声音，并确认电脑扬声器未静音。</p>
  <p v-if="audio.storageError.value" role="alert" class="notice">浏览器未允许保存偏好，设置未保存，请检查存储权限或空间后重试。</p>
  <p v-if="audio.playbackError.value" role="alert" class="notice">{{audio.playbackError.value}}</p>
  <details class="sound-help"><summary>声音与颜色说明</summary><ul><li>内置音频仅保留 Buzzer4（提示）、Buzzer1（警告）、Alarm2（严重）和 Siren1（备用警笛）。仅用于网页辅助提示，不替代现场声光报警器。</li><li>持续模式下内置和自定义音频均完整循环，直到告警恢复、确认、静音或关声；关闭面板不会静音。</li><li>颜色用于等级标识、列表和卡片边框，等级文字保留；严重告警仅边框闪烁。</li><li>网页需保持打开；系统静音、休眠和浏览器限制可能影响播放。旧版合成音色自动对应到本等级默认音频，音量、颜色和声音开关保持不变。</li></ul></details>
 </section>
 <ConfirmationDialog :open="leaveOpen" title="声音与颜色设置尚未保存" message="是否保存本次修改？" detail="保存后应用新的声音和颜色；放弃修改将保留原设置。" confirm-text="保存并离开" cancel-text="继续编辑" :confirm-disabled="busy||conflict" @cancel="finishLeave(false)" @confirm="saveAndLeave">
  <p v-if="busy" role="status">音频正在校验，请稍候，或放弃修改。</p>
  <p v-if="notice||conflict" role="alert">{{conflict?'其他标签页已更新设置，请放弃本页修改后重新编辑。':notice}}</p>
  <template #actions><button type="button" @click="discardAndLeave">放弃修改</button></template>
 </ConfirmationDialog>
</template>
<style scoped>
.sound-config-body .upload-button input:disabled{opacity:0}
.sound-master-control{display:inline-flex;align-items:center;gap:9px;flex-shrink:0;align-self:flex-start;padding:2px 0;cursor:pointer;white-space:nowrap;min-height:30px}.sound-master-state{color:#64748b;min-width:36px}.sound-master-state.on{color:#15803d}.sound-master-switch{position:relative;display:inline-flex;width:40px;height:22px;flex-shrink:0}.sound-master-switch input{position:absolute;inset:0;width:100%;height:100%;padding:0;margin:0;opacity:0;z-index:1;cursor:pointer}.sound-master-switch i{width:40px;height:22px;display:block;border-radius:12px;background:#94a3b8;transition:background .15s}.sound-master-switch i:after{content:'';display:block;width:18px;height:18px;position:absolute;top:2px;left:2px;background:white;border-radius:50%;box-shadow:0 1px 3px #0002;transition:transform .15s}.sound-master-switch input:checked+i{background:#16a34a}.sound-master-switch input:checked+i:after{transform:translateX(18px)}.sound-master-switch input:focus-visible+i{outline:3px solid #93c5fd;outline-offset:3px}.sound-config-body{border:0;padding:0;margin:0;min-width:0}.sound-config-body.is-disabled{filter:grayscale(1);opacity:.6}.sound-config-body:disabled :is(button,input,select,.upload-button){cursor:not-allowed}.sound-off-notice{margin:14px 0 0;padding:10px 12px;background:#f1f5f9;border:1px solid #e2e8f0;border-radius:7px;color:#475569;font-size:12px;line-height:1.7}@media(prefers-reduced-motion:reduce){.sound-master-switch i,.sound-master-switch i:after{transition:none}}

.sound-save-bar{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-top:14px;font-size:12px;color:#64748b}.sound-save-bar>div{display:flex;gap:8px}.sound-save-bar button{height:34px;padding:0 14px;border:1px solid #dbe5f2;border-radius:6px;background:#fff;color:#203047;cursor:pointer}.sound-save-bar button.primary{background:#1677ff;border-color:#1677ff;color:white}.sound-save-bar button:disabled{opacity:.5;cursor:not-allowed}

.alarm-sound-settings{padding:20px;color:#203047}.sound-heading{display:flex;justify-content:space-between;gap:20px;align-items:start}.sound-heading h3{margin:0 0 6px;font-size:16px}.sound-heading p{margin:0;color:#64748b;font-size:12px;line-height:1.7}.alarm-sound-settings label{font-size:12px;min-width:0}.alarm-sound-settings select{height:34px;min-width:0;width:100%;border:1px solid #dbe5f2;border-radius:7px;background:white;padding:0 8px;color:#203047}.level-settings{margin-top:16px;padding:10px;border:1px solid #dbe5f2;border-radius:8px;background:#fff}.level-settings-head,.level-settings-row{display:grid;grid-template-columns:88px minmax(200px,1fr) 142px 120px 118px 110px;gap:12px;align-items:start}.level-settings-head{font-size:11px;color:#64748b;padding:2px 0 8px}.level-settings-row{padding:10px 0;border-top:1px solid #edf1f6}.level-settings-row>label,.row-actions{min-height:34px;display:flex;align-items:center;gap:7px}.level-toggle input{width:16px;height:16px;margin:0;accent-color:#1677ff;flex-shrink:0}.level-toggle strong{font-size:12px}.tone-cell{min-width:0}.upload-line{display:flex;align-items:center;gap:8px;margin-top:5px;min-width:0}.upload-button{position:relative;color:#1765c1;cursor:pointer;font-size:11px;flex-shrink:0}.upload-button input{position:absolute;inset:0;opacity:0;width:100%;cursor:pointer}.upload-button:focus-within{outline:2px solid #93c5fd;outline-offset:2px}.upload-button.busy{color:#94a3b8}.file-name{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:#64748b;font-size:11px}.text-button{border:0;background:none;padding:0;color:#64748b;cursor:pointer;white-space:nowrap;font-size:11px}.text-button:hover{color:#1677ff}.volume-cell input{width:100%;min-width:0;accent-color:#1677ff;margin:0}.volume-cell output{font-size:11px;color:#526780;min-width:30px}.color-control input{width:32px;height:28px;padding:2px;border:1px solid #dbe5f2;border-radius:5px;background:white;cursor:pointer;flex-shrink:0}.color-control code{font-size:11px;color:#526780}.preview,.stop-preview{height:32px;padding:0 12px;border:1px solid #b9d3f7;border-radius:6px;background:#eff6ff;color:#1765c1;cursor:pointer;white-space:nowrap}.row-actions{gap:8px}.upload-hint{font-size:11px;line-height:1.6;color:#64748b;margin:7px 0 0}.sound-footer{display:flex;align-items:center;gap:12px;margin-top:12px}.sound-footer span{font-size:12px;color:#64748b}.sound-help{font-size:12px;color:#64748b;margin-top:14px}.sound-help summary{cursor:pointer}.sound-help li{line-height:1.8}.sound-help ul{padding-left:20px}.notice{background:#fff7e6;padding:10px;border-radius:6px;font-size:12px}.upload-error{color:#dc2626;font-size:11px;margin:4px 0;line-height:1.5}.mobile-label{display:none}
@media(max-width:1200px){.level-settings-head,.level-settings-row{grid-template-columns:76px minmax(160px,1fr) 130px 90px 98px 100px;gap:8px}.color-control{flex-wrap:wrap}.color-control code{font-size:10px}.preview{padding:0 8px}}
@media(max-width:1000px){.level-settings-head{display:none}.level-settings-row{grid-template-columns:76px minmax(0,1fr) minmax(0,1fr);gap:10px 12px}.level-settings-row:first-of-type{border-top:0}.tone-cell{grid-column:2/-1}.level-settings-row>label:not(.level-toggle){display:grid;gap:5px}.mobile-label{display:block;font-size:11px;color:#64748b}.volume-cell{grid-column:2}.color-control{grid-column:3}.color-control code{font-size:11px}.row-actions{grid-column:2/-1}.level-settings-row .color-control{display:flex;flex-wrap:wrap}.color-control .mobile-label{width:100%}}
@media(max-width:600px){.alarm-sound-settings{padding:14px}.sound-heading{flex-direction:column;gap:10px}.sound-master-control{align-self:flex-end}.level-settings-row{grid-template-columns:repeat(2,minmax(0,1fr))}.level-toggle{grid-column:1/-1}.tone-cell{grid-column:1/-1}.volume-cell{grid-column:2}.color-control{grid-column:1}.row-actions{grid-column:2;justify-content:flex-end}.sound-footer{align-items:flex-start}.sound-footer span{line-height:1.6}}
</style>
