<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props=withDefaults(defineProps<{
  open:boolean
  title:string
  message:string
  detail?:string
  confirmText?:string
  cancelText?:string
  danger?:boolean
  confirmDisabled?:boolean
  panelClass?:string
  kind?:''|'license'
  fields?:readonly {label:string;value:string}[]
}>(),{detail:'',confirmText:'确认',cancelText:'取消',danger:false,confirmDisabled:false,panelClass:''})
const emit=defineEmits<{cancel:[];confirm:[]}>()
const confirmButton=ref<HTMLButtonElement>()

function cancel(){emit('cancel')}
function confirm(){if(!props.confirmDisabled)emit('confirm')}
function onKeydown(event:KeyboardEvent){
  if(!props.open||event.key!=='Escape')return
  event.preventDefault()
  cancel()
}

watch(()=>props.open,open=>{if(open)nextTick(()=>confirmButton.value?.focus())},{immediate:true})
onMounted(()=>window.addEventListener('keydown',onKeydown))
onBeforeUnmount(()=>window.removeEventListener('keydown',onKeydown))
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="restart-modal global-confirm-modal" @click.self="cancel">
      <div class="dialog-panel confirmation-dialog" :class="[panelClass,{'license-confirm-dialog':kind==='license'}]" role="alertdialog" aria-modal="true" :aria-label="title">
        <div class="confirmation-dialog-heading">
          <span class="confirmation-dialog-icon" :class="{ danger }" aria-hidden="true">
            <svg v-if="kind==='license'" viewBox="0 0 24 24"><path d="m12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6l8-3Z M8 12l3 3 5-6"/></svg>
            <svg v-else viewBox="0 0 24 24"><path d="M12 8v5m0 3.5v.01M10.3 3.9 2.4 17.6A2 2 0 0 0 4.1 20h15.8a2 2 0 0 0 1.7-2.4L13.7 3.9a2 2 0 0 0-3.4 0Z"/></svg>
          </span>
          <div>
            <h3>{{title}}</h3>
            <p>{{message}}</p>
          </div>
        </div>
        <dl v-if="fields?.length" class="confirmation-fields"><div v-for="field in fields" :key="field.label"><dt>{{field.label}}</dt><dd :class="{'perpetual':field.value==='永久有效'}">{{field.value}}</dd></div></dl>
        <p v-if="detail" class="global-confirm-detail">{{detail}}</p>
        <div v-if="$slots.default" class="confirmation-dialog-extra"><slot/></div>
        <div class="dialog-actions">
          <button type="button" @click="cancel">{{cancelText}}</button>
          <slot name="actions"/>
          <button ref="confirmButton" type="button" :disabled="confirmDisabled" :class="{danger,primary:!danger}" @click="confirm">{{confirmText}}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.global-confirm-modal { z-index:2000; }
.confirmation-dialog .dialog-actions { flex-wrap:wrap; }
.global-confirm-modal .license-confirm-dialog { width:min(460px,calc(100vw - 32px)); }
.license-confirm-dialog .confirmation-dialog-heading { padding:24px 24px 18px;grid-template-columns:44px minmax(0,1fr);border:0; }
.license-confirm-dialog .confirmation-dialog-heading h3 { font-size:19px; }
.license-confirm-dialog .confirmation-dialog-icon { width:44px;height:44px;background:#edf4ff;color:#2474ed; }
.confirmation-fields { display:grid;gap:14px;margin:0 24px;padding:18px;border:1px solid #e4ebf5;border-radius:12px;background:#f8faff; }
.confirmation-fields > div { display:grid;grid-template-columns:90px minmax(0,1fr);align-items:start;gap:12px; }
.confirmation-fields dt { color:#8090a6;font-size:13px; }
.confirmation-fields dd { margin:0;color:#243955;font-size:13px;font-weight:600;overflow-wrap:anywhere; }
.confirmation-fields dd.perpetual { color:#15965a; }
.license-confirm-dialog .global-confirm-detail { margin:14px 24px 20px!important;padding:0;border:0;background:none;color:#8090a6;font-size:12px;line-height:1.7; }
.license-confirm-dialog .dialog-actions { padding:16px 24px; }
@media(max-width:640px){.confirmation-fields{margin:0 16px;padding:16px}.license-confirm-dialog .confirmation-dialog-heading{padding:20px 16px 16px}.license-confirm-dialog .global-confirm-detail{margin:14px 16px 18px!important}.license-confirm-dialog .dialog-actions{padding:14px 16px}}
</style>
