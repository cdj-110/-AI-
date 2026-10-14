<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { Edit, Plus, RefreshRight } from '@element-plus/icons-vue';
import { ElMessage } from 'element-plus';
import { apiRequest } from '../api/request';

interface Addon { id:string;code:string;name:string;description?:string;resourceType:string;quantity:number;priceCents:number;quoteOnly:boolean;pricePending:boolean;enabled:boolean;sortOrder:number }

const addons = ref<Addon[]>([]);
const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const editingId = ref('');
const status = ref('ALL');
const form = reactive({code:'',name:'',description:'',resourceType:'POINT',quantity:5000,price:0,quoteOnly:false,pricePending:false,enabled:true,sortOrder:10});
const filteredAddons = computed(() => addons.value.filter(addon => !['ENABLED', 'DISABLED'].includes(status.value) || addon.enabled === (status.value === 'ENABLED')));

async function load(){loading.value=true;try{addons.value=await apiRequest({url:'/api/subscriptions/addons',method:'GET'})}finally{loading.value=false}}
function openEditor(item?:Addon){editingId.value=item?.id||'';Object.assign(form,item?{...item,price:item.priceCents/100}:{code:'',name:'',description:'',resourceType:'POINT',quantity:5000,price:0,quoteOnly:false,pricePending:false,enabled:true,sortOrder:(addons.value.length+1)*10});dialogVisible.value=true}
async function save(){if(!form.code.trim()||!form.name.trim())return ElMessage.warning('请填写扩容包编码和名称');saving.value=true;try{const data={...form,priceCents:Math.round(form.price*100)} as Record<string,unknown>;delete data.price;await apiRequest({url:editingId.value?`/api/subscriptions/addons/${editingId.value}`:'/api/subscriptions/addons',method:editingId.value?'PATCH':'POST',data});ElMessage.success(editingId.value?'扩容包已更新':'扩容包已创建');dialogVisible.value=false;await load()}finally{saving.value=false}}
const money=(cents:number)=>`¥${(cents/100).toLocaleString('zh-CN',{minimumFractionDigits:2})}`;
const resourceName=(value:string)=>({DEVICE:'设备',POINT:'云点位',SCADA:'组态项目',USER:'用户',HISTORY:'历史保存（月）'}[value]||value);
onMounted(load);
</script>

<template>
  <div>
    <div class="page-actions">
      <div><h1 class="page-title">扩容包管理</h1><p class="page-description">管理资源额度及历史存储扩展；历史每份增加3个月，随购买时订阅到期，价格待定时禁止购买</p></div>
      <div class="header-actions">
        <el-select v-model="status" aria-label="销售状态" title="按销售状态筛选">
          <el-option label="全部状态" value="ALL"/>
          <el-option label="销售中" value="ENABLED"/>
          <el-option label="已停用" value="DISABLED"/>
        </el-select>
        <el-button :icon="RefreshRight" @click="load">刷新</el-button>
        <el-button type="primary" :icon="Plus" @click="openEditor()">新增扩容包</el-button>
      </div>
    </div>
    <section class="table-panel" v-loading="loading"><el-table :data="filteredAddons" :empty-text="['ENABLED','DISABLED'].includes(status)?'当前状态下暂无扩容包':'暂无扩容包'"><el-table-column prop="name" label="扩容包" min-width="210"/><el-table-column prop="code" label="编码" min-width="170"/><el-table-column label="资源类型" width="120"><template #default="{row}">{{resourceName(row.resourceType)}}</template></el-table-column><el-table-column label="扩容量" width="130"><template #default="{row}">+{{row.quantity.toLocaleString()}}</template></el-table-column><el-table-column label="销售方式" width="120"><template #default="{row}"><el-tag :type="row.quoteOnly?'warning':'primary'" effect="plain">{{row.quoteOnly?'商务报价':'在线下单'}}</el-tag></template></el-table-column><el-table-column label="价格" width="140"><template #default="{row}">{{row.pricePending?'价格待定':row.quoteOnly?'商务报价':money(row.priceCents)}}</template></el-table-column><el-table-column label="状态" width="110"><template #default="{row}"><el-tag :type="row.enabled?'success':'info'">{{row.enabled?'销售中':'已停用'}}</el-tag></template></el-table-column><el-table-column label="操作" width="90" fixed="right"><template #default="{row}"><el-button link type="primary" :icon="Edit" @click="openEditor(row)">编辑</el-button></template></el-table-column></el-table></section>

    <el-dialog v-model="dialogVisible" :title="editingId?'编辑扩容包':'新增扩容包'" width="580px"><el-form label-position="top"><div class="form-grid"><el-form-item label="扩容包编码" required><el-input v-model="form.code"/></el-form-item><el-form-item label="扩容包名称" required><el-input v-model="form.name"/></el-form-item></div><el-form-item label="说明"><el-input v-model="form.description" type="textarea" :rows="2"/></el-form-item><div class="form-grid"><el-form-item label="资源类型"><el-select v-model="form.resourceType" @change="form.resourceType==='HISTORY' && Object.assign(form,{quantity:3,pricePending:true,enabled:false})"><el-option label="设备" value="DEVICE"/><el-option label="云点位" value="POINT"/><el-option label="组态项目" value="SCADA"/><el-option label="用户" value="USER"/><el-option label="历史保存（月）" value="HISTORY"/></el-select></el-form-item><el-form-item label="每份扩容量"><el-input-number v-model="form.quantity" :min="1" controls-position="right"/></el-form-item><el-form-item label="每份价格（元）"><el-input-number v-model="form.price" :disabled="form.quoteOnly||form.pricePending" :min="0" :precision="2" controls-position="right"/></el-form-item><el-form-item label="定价状态"><el-switch v-model="form.pricePending" active-text="价格待定（不可购买）"/></el-form-item><el-form-item label="销售方式"><el-switch v-model="form.quoteOnly" active-text="商务报价" inactive-text="在线下单"/></el-form-item><el-form-item label="状态"><el-switch v-model="form.enabled" active-text="销售中" inactive-text="已停用"/></el-form-item><el-form-item label="排序"><el-input-number v-model="form.sortOrder" :min="0" :max="10000" controls-position="right"/></el-form-item></div></el-form><template #footer><el-button @click="dialogVisible=false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存</el-button></template></el-dialog>
  </div>
</template>

<style scoped>
.header-actions{display:flex;align-items:center;gap:10px}.header-actions>.el-select{width:150px}.header-actions .el-button+.el-button{margin-left:0}.form-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}.form-grid :deep(.el-input-number),.form-grid :deep(.el-select){width:100%}@media(max-width:700px){.page-actions{align-items:flex-start;flex-direction:column;gap:16px}.header-actions{width:100%;flex-wrap:wrap}.header-actions>.el-select{flex:1;min-width:140px}.form-grid{grid-template-columns:1fr}}
</style>
