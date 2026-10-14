<script setup lang="ts">
import {computed} from 'vue'
import {dataTypeLabel} from '../dataTypeLabels.ts'
import {dlt645Formats,dlt645ValueTypes,dlt645Selection,dlt645ConfiguredType,selectDLT645Option,selectDLT645Type,type DLT645Item} from '../dlt645PointOptions.ts'
const props=defineProps<{point:any;items:DLT645Item[];kind:'format'|'type';version?:string}>()
const groups=computed(()=>[...new Set(props.items.map(item=>item.category))])
const current=computed(()=>props.kind==='format'?dlt645Selection(props.point):dlt645ConfiguredType(props.point,props.items))
const unknownItem=computed(()=>props.point.dlt645?.itemKey&&!props.items.some(item=>item.key===props.point.dlt645.itemKey))
function change(event:Event){
  const value=(event.target as HTMLSelectElement).value
  if(props.kind==='format')selectDLT645Option(props.point,value,props.items,props.version)
  else selectDLT645Type(props.point,value,props.items)
}
</script>
<template>
  <select v-if="kind==='format'" :value="current" aria-label="数据项 / 格式" @change="change">
    <option v-if="unknownItem" :value="current" disabled>未加载的数据项：{{point.dlt645.itemKey}}</option>
    <optgroup label="手动 DI / 数据格式"><option v-for="format in dlt645Formats" :key="format.key" :value="'format:'+format.key">{{format.label}}</option></optgroup>
    <optgroup v-for="category in groups" :key="category" :label="category"><option v-for="item in items.filter(item=>item.category===category)" :key="item.key" :value="'item:'+item.key" :disabled="(version==='2007'&&!item.di2007)||(version==='1997'&&!item.di1997)">{{item.name}}</option></optgroup>
  </select>
  <select v-else :value="current" aria-label="值类型" :disabled="!!point.dlt645?.itemKey" :title="point.dlt645?.itemKey?'标准数据项类型由字典确定；如需自定义，请先选择手动格式':'修改类型会同步切换解析格式，请核对数据长度和小数位'" @change="change">
    <option v-if="!dlt645ValueTypes.includes(current)" :value="current" disabled>{{dataTypeLabel(current)||'请选择格式'}}</option>
    <option v-for="type in dlt645ValueTypes" :key="type" :value="type">{{dataTypeLabel(type)}}</option>
  </select>
</template>
