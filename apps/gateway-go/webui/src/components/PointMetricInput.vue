<script setup lang="ts">
import { ref, watch } from 'vue'

const props=defineProps<{modelValue:string;disabled?:boolean}>()
const emit=defineEmits<{'update:modelValue':[value:string]}>()
const draft=ref(props.modelValue)
watch(()=>props.modelValue,value=>{draft.value=value})

function commit(){
  if(props.disabled)return
  const value=draft.value.trim()
  // An empty edit cancels instead of unexpectedly allocating another identifier.
  draft.value=value||props.modelValue
  if(value&&value!==props.modelValue)emit('update:modelValue',value)
}
function finish(event:KeyboardEvent){
  if(event.isComposing)return
  event.preventDefault()
  ;(event.target as HTMLInputElement).blur()
}
function cancel(event:KeyboardEvent){
  event.preventDefault()
  draft.value=props.modelValue
  ;(event.target as HTMLInputElement).blur()
}
function selectAll(event:FocusEvent){(event.target as HTMLInputElement).select()}
</script>

<template>
  <input v-model="draft" class="mono" :disabled="disabled" :title="modelValue" aria-label="点位标识符" spellcheck="false" @focus="selectAll" @blur="commit" @keydown.enter="finish" @keydown.esc="cancel"/>
</template>
