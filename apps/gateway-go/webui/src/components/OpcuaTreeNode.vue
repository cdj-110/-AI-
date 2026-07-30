<script setup lang="ts">
import { computed } from 'vue'

const props=defineProps<{node:any;selected:string[];filter:string}>()
const emit=defineEmits<{expand:[node:any];toggle:[node:any];folder:[node:any]}>()
const visible=computed(()=>{
  const query=props.filter.trim().toLowerCase()
  if(!query)return true
  const own=`${props.node.displayName||''} ${props.node.browseName||''} ${props.node.nodeId||''}`.toLowerCase().includes(query)
  const child=(props.node.children||[]).some((item:any)=>`${item.displayName||''} ${item.browseName||''} ${item.nodeId||''}`.toLowerCase().includes(query))
  return own||child
})
</script>

<template>
  <div v-if="visible" class="opcua-browser-node">
    <div class="opcua-browser-row">
      <button class="opcua-browser-expand" :class="{expanded:node.expanded,leaf:!node.hasChildren}" type="button" :disabled="node.loading" @click="emit('expand',node)"><span class="opcua-browser-chevron"/></button>
      <input v-if="node.selectable" class="opcua-browser-check" type="checkbox" :checked="selected.includes(node.nodeId)" @change="emit('toggle',node)"/>
      <span v-else class="opcua-browser-check-placeholder"/>
      <span class="opcua-browser-label"><strong class="opcua-browser-name">{{node.displayName||node.browseName||node.nodeId}}</strong><small class="opcua-browser-meta">{{node.nodeId}}</small></span>
      <span class="opcua-browser-kind">{{node.selectable?(node.dataType||'变量'):(node.nodeClass||'目录')}}</span>
      <button v-if="node.hasChildren&&!node.selectable" class="opcua-browser-pick-folder" type="button" :disabled="node.selecting" @click="emit('folder',node)">{{node.selecting?'选择中…':'选择下级变量'}}</button>
    </div>
    <div v-if="node.expanded" class="opcua-browser-children">
      <p v-if="node.loading" class="opcua-browser-empty">加载中…</p>
      <OpcuaTreeNode v-for="child in node.children||[]" :key="child.nodeId" :node="child" :selected="selected" :filter="filter" @expand="emit('expand',$event)" @toggle="emit('toggle',$event)" @folder="emit('folder',$event)"/>
      <p v-if="node.loaded&&!node.loading&&!(node.children||[]).length" class="opcua-browser-empty">此节点下没有子节点</p>
    </div>
  </div>
</template>
