<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { Plus, Search, RefreshRight } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { apiRequest } from '../api/request';
import DeviceLinkageEditor from '../components/DeviceLinkageEditor.vue';
interface Rule { id:string;name:string;sourceName:string;sourceMetric:string;operator:string;threshold:number;targetName:string;targetMetric:string;actionValue:unknown;enabled:boolean;lastError?:string }
interface Run {id:string;sampleAt:string;observedValue:number;actionValue:unknown;status:string;message?:string;commandId?:string;createdAt:string}
const items=ref<Rule[]>([]),total=ref(0),page=ref(1),keyword=ref(''),loading=ref(false),changing=ref('');
const editing=ref(false),editRule=ref<Rule>(),viewing=ref(false),viewRule=ref<Rule>(),runs=ref<Run[]>([]),runTotal=ref(0),runPage=ref(1),runLoading=ref(false);
const operators:Record<string,string>={GT:'>',GTE:'≥',LT:'<',LTE:'≤',EQ:'=',NE:'≠'};
const statuses:Record<string,string>={DISPATCHING:'下发状态待确认',SENT:'等待回执',SUCCEEDED:'网关已执行',FAILED:'失败',TIMEOUT:'结果未知',SKIPPED:'已阻止',THROTTLED:'频率保护',CANCELLED:'未发送'};
const date=(v:string)=>new Date(v).toLocaleString('zh-CN',{timeZone:'Asia/Shanghai'});
let generation=0,runGeneration=0;
async function load(next=page.value){const seq=++generation;loading.value=true;try{const data=await apiRequest<{items:Rule[];total:number;page:number}>({url:'/api/device-linkage',method:'GET',params:{page:next,keyword:keyword.value}});if(seq===generation){items.value=data.items;total.value=data.total;page.value=data.page;}}finally{if(seq===generation)loading.value=false;}}
async function saved(){editing.value=false;await load(1);}
async function toggle(rule:Rule){
  try{
    await ElMessageBox.confirm(rule.enabled?'停用后不再产生新指令，已进入下发流程的指令可能仍会执行。':`启用“${rule.name}”后，${rule.sourceName} / ${rule.sourceMetric} ${operators[rule.operator]} ${rule.threshold} 时将自动向 ${rule.targetName} / ${rule.targetMetric} 下发 ${JSON.stringify(rule.actionValue)}，不逐次弹框确认。请确认设备、值含义和现场安全。`,'设备联动授权',{type:'warning',confirmButtonText:rule.enabled?'停用规则':'授权并启用',cancelButtonText:'取消'});
    changing.value=rule.id;await apiRequest({url:`/api/device-linkage/${rule.id}/enabled`,method:'PATCH',data:{enabled:!rule.enabled,confirmAutomaticControl:!rule.enabled}});ElMessage.success(rule.enabled?'已停用':'已启用');await load();
  }catch(e){if(e!=='cancel'&&e!=='close')throw e;}finally{changing.value='';}
}
async function remove(rule:Rule){try{await ElMessageBox.confirm(`删除“${rule.name}”规则？只删除配置，不改变设备实际状态。`,'删除联动规则',{type:'warning',confirmButtonText:'删除',cancelButtonText:'取消'});await apiRequest({url:`/api/device-linkage/${rule.id}`,method:'DELETE'});await load();}catch(e){if(e!=='cancel'&&e!=='close')throw e;}}
async function loadRuns(next=1){if(!viewRule.value)return;const id=viewRule.value.id,seq=++runGeneration;runLoading.value=true;try{const data=await apiRequest<{items:Run[];total:number;page:number}>({url:`/api/device-linkage/${id}/runs`,method:'GET',params:{page:next}});if(seq===runGeneration){runs.value=data.items;runTotal.value=data.total;runPage.value=data.page;}}finally{if(seq===runGeneration)runLoading.value=false;}}
function view(rule:Rule){viewRule.value=rule;runs.value=[];runTotal.value=0;viewing.value=true;void loadRuns();}
onMounted(()=>{void load();});
</script>
<template>
<div><div class="page-actions"><div><h1 class="page-title">设备联动</h1><p class="page-description">云端按实时点位条件自动下发，独立于组态页面运行</p></div><el-button type="primary" :icon="Plus" @click="editRule=undefined;editing=true">新增联动规则</el-button></div>
<el-alert title="云端联动依赖云连接，不替代设备安全保护或硬件联锁" type="info" :closable="false" show-icon class="linkage-banner" />
<section class="table-panel"><div class="toolbar"><el-input v-model="keyword" placeholder="搜索规则名称" clearable :prefix-icon="Search" @keyup.enter="load(1)"/><el-button @click="load(1)">查询</el-button><el-button :icon="RefreshRight" @click="load()">刷新</el-button><span>共 {{total}} 条规则</span></div>
<el-table :data="items" v-loading="loading" empty-text="暂无联动规则，点击右上角新增" stripe><el-table-column prop="name" label="规则名称" min-width="170"/><el-table-column label="满足条件" min-width="270"><template #default="{row}">{{row.sourceName}} / {{row.sourceMetric}} {{operators[row.operator]}} {{row.threshold}}</template></el-table-column><el-table-column label="执行动作" min-width="260"><template #default="{row}">{{row.targetName}} / {{row.targetMetric}} → {{JSON.stringify(row.actionValue)}}</template></el-table-column><el-table-column label="状态" min-width="170"><template #default="{row}"><el-tag :type="row.enabled?'success':'info'">{{row.enabled?'已启用':'已停用'}}</el-tag><div v-if="row.lastError" class="error">{{row.lastError}}</div></template></el-table-column><el-table-column label="操作" width="270" fixed="right"><template #default="{row}"><el-button link type="primary" :loading="changing===row.id" :disabled="!!changing" @click="toggle(row)">{{row.enabled?'停用':'启用'}}</el-button><el-button link type="primary" :disabled="row.enabled" @click="editRule=row;editing=true">编辑</el-button><el-button link type="primary" @click="view(row)">执行记录</el-button><el-button link type="danger" :disabled="row.enabled" @click="remove(row)">删除</el-button></template></el-table-column></el-table>
<div class="footer"><span>仅租户管理员管理；新规则默认停用，启用后首次有效满足触发一次。编辑和删除前请先停用。</span><el-pagination v-model:current-page="page" :page-size="20" :total="total" layout="total, prev, pager, next" @current-change="load" /></div></section>
<el-dialog v-model="editing" :title="editRule?'编辑联动规则':'新增联动规则'" width="min(820px,94vw)" top="5vh" class="linkage-editor-dialog" destroy-on-close :close-on-click-modal="false"><DeviceLinkageEditor v-if="editing" :rule="editRule" @saved="saved" @cancel="editing=false"/></el-dialog>
<el-dialog v-model="viewing" :title="`${viewRule?.name||''} · 执行记录`" width="min(1050px,96vw)" destroy-on-close @closed="runGeneration++"><el-alert title="已发送不代表执行成功；结果未知时请核实现场状态和网关回执，不要盲目重复下发。" type="info" :closable="false"/><div class="record-toolbar"><el-button :icon="RefreshRight" @click="loadRuns(runPage)">刷新回执</el-button></div><el-table :data="runs" v-loading="runLoading" empty-text="暂无触发记录"><el-table-column label="触发时间" min-width="185"><template #default="{row}">{{date(row.sampleAt)}}</template></el-table-column><el-table-column prop="observedValue" label="触发值" width="100"/><el-table-column label="下发值" width="100"><template #default="{row}">{{JSON.stringify(row.actionValue)}}</template></el-table-column><el-table-column label="执行状态" width="150"><template #default="{row}">{{statuses[row.status]||row.status}}</template></el-table-column><el-table-column prop="message" label="回执 / 说明" min-width="260"/></el-table><div class="footer"><el-pagination v-model:current-page="runPage" :page-size="20" :total="runTotal" layout="total, prev, pager, next" @current-change="loadRuns"/></div></el-dialog>
</div>
</template>
<style>.linkage-editor-dialog .el-dialog__body{overflow:hidden}.linkage-editor-dialog .el-dialog__header{padding-bottom:14px}</style>
<style scoped>.linkage-banner{margin-bottom:20px}.toolbar{display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin-bottom:20px}.toolbar .el-input{max-width:330px}.toolbar>span{margin-left:auto;color:#7f8da2;font-size:13px}.footer{display:flex;justify-content:space-between;align-items:center;gap:16px;flex-wrap:wrap;margin-top:20px}.footer>span{font-size:12px;color:#7f8da2;line-height:1.7}.error{font-size:12px;color:#e65b5b;margin-top:6px}.record-toolbar{margin:16px 0}@media(max-width:650px){.page-actions{flex-direction:column;align-items:flex-start;gap:12px}.toolbar .el-input{max-width:none;width:100%}}</style>
