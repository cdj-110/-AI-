<script setup lang="ts">
import { computed } from 'vue'

const props=defineProps<{node:any;selected:string[];filter:string}>()
const emit=defineEmits<{expand:[node:any];toggle:[node:any];folder:[node:any]}>()

function matches(node:any,query:string):boolean{
  const own=`${node.name||''} ${node.objectRef||''} ${node.fc||''} ${node.dataType||''}`.toLowerCase()
  return own.includes(query)||(node.children||[]).some((child:any)=>matches(child,query))
}

const query=computed(()=>props.filter.trim().toLowerCase())
const visible=computed(()=>!query.value||matches(props.node,query.value))
const showChildren=computed(()=>props.node.expanded||Boolean(query.value))
</script>

<template>
  <div v-if="visible" class="opcua-browser-node iec61850-browser-node">
    <div class="opcua-browser-row">
      <button class="opcua-browser-expand" :class="{expanded:showChildren,leaf:!node.hasChildren}" type="button" @click="emit('expand',node)"><span class="opcua-browser-chevron"/></button>
      <input v-if="node.selectable" class="opcua-browser-check" type="checkbox" :checked="selected.includes(node.key)" @change="emit('toggle',node)"/>
      <span v-else class="opcua-browser-check-placeholder"/>
      <span class="opcua-browser-label">
        <strong class="opcua-browser-name">{{node.name||node.objectRef}}</strong>
        <small class="opcua-browser-meta">{{node.objectRef}}</small>
      </span>
      <span class="opcua-browser-kind">{{node.selectable?`${node.fc||'-'} · ${node.dataType||'auto'}`:(node.kind||'目录')}}</span>
      <button v-if="node.hasChildren&&!node.selectable" class="opcua-browser-pick-folder" type="button" @click="emit('folder',node)">选择下级点位</button>
    </div>
    <div v-if="showChildren&&node.hasChildren" class="opcua-browser-children">
      <Iec61850TreeNode v-for="child in node.children" :key="child.key" :node="child" :selected="selected" :filter="filter" @expand="emit('expand',$event)" @toggle="emit('toggle',$event)" @folder="emit('folder',$event)"/>
    </div>
  </div>
</template>
