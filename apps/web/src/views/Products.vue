<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { Plus, RefreshRight, Search } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { apiRequest } from '../api/request';
import { type ProductModel, supportText } from '../types/products';

type SupportChoice = 'UNKNOWN' | 'YES' | 'NO';
const products = ref<ProductModel[]>([]);
const loading = ref(false);
const saving = ref(false);
const keyword = ref('');
const dialogVisible = ref(false);
const editingCode = ref('');
const form = reactive({ code: '', name: '', ethernetPortCount: null as number | null, rs232PortCount: null as number | null, rs485PortCount: null as number | null, canPortCount: null as number | null, supports4G: 'UNKNOWN' as SupportChoice, supportsWifi: 'UNKNOWN' as SupportChoice, enabled: true, description: '' });
const filtered = computed(() => products.value.filter(product => `${product.code} ${product.name}`.toLowerCase().includes(keyword.value.trim().toLowerCase())));
const enabledCount = computed(() => products.value.filter(product => product.enabled).length);
const unknownCount = computed(() => products.value.filter(product => product.ethernetPortCount == null || product.rs232PortCount == null || product.rs485PortCount == null || product.canPortCount == null || product.supports4G == null || product.supportsWifi == null).length);
const supportChoices = [{ label: '待确认', value: 'UNKNOWN' }, { label: '支持', value: 'YES' }, { label: '不支持', value: 'NO' }];
const choice = (value: boolean | null): SupportChoice => value === true ? 'YES' : value === false ? 'NO' : 'UNKNOWN';
const flag = (value: SupportChoice) => value === 'YES' ? true : value === 'NO' ? false : null;

async function load() {
  loading.value = true;
  try { products.value = await apiRequest<ProductModel[]>({ url: '/api/products', method: 'GET' }); }
  finally { loading.value = false; }
}

function open(product?: ProductModel) {
  editingCode.value = product?.code || '';
  Object.assign(form, product ? {
    ...product, supports4G: choice(product.supports4G), supportsWifi: choice(product.supportsWifi), description: product.description || '',
  } : { code: '', name: '', ethernetPortCount: null, rs232PortCount: null, rs485PortCount: null, canPortCount: null, supports4G: 'UNKNOWN', supportsWifi: 'UNKNOWN', enabled: true, description: '' });
  dialogVisible.value = true;
}

async function save() {
  const code = form.code.trim().toUpperCase();
  if (!/^[A-Z0-9][A-Z0-9_-]{0,31}$/.test(code)) { ElMessage.warning('型号限 32 位字母、数字、横线或下划线，且以字母或数字开头'); return; }
  if (!form.name.trim()) { ElMessage.warning('请填写产品名称'); return; }
  for (const count of [form.ethernetPortCount, form.rs232PortCount, form.rs485PortCount, form.canPortCount]) {
    if (count !== null && count !== undefined && (!Number.isInteger(count) || count < 0 || count > 64)) { ElMessage.warning('接口数量须为 0～64 的整数，留空表示待确认'); return; }
  }
  saving.value = true;
  try {
    const data = { name: form.name.trim(), ethernetPortCount: form.ethernetPortCount ?? null, rs232PortCount: form.rs232PortCount ?? null, rs485PortCount: form.rs485PortCount ?? null, canPortCount: form.canPortCount ?? null, supports4G: flag(form.supports4G), supportsWifi: flag(form.supportsWifi), enabled: form.enabled, description: form.description.trim() || null };
    await apiRequest({ url: editingCode.value ? `/api/products/${encodeURIComponent(editingCode.value)}` : '/api/products', method: editingCode.value ? 'PATCH' : 'POST', data: editingCode.value ? data : { ...data, code } });
    ElMessage.success(editingCode.value ? '产品规格已更新' : '产品型号已新增');
    dialogVisible.value = false;
    await load();
  } finally { saving.value = false; }
}

async function remove(product: ProductModel) {
  try { await ElMessageBox.confirm(`确认删除产品型号 ${product.code} 吗？已关联出厂网关的型号不能删除，可改为停用。`, '删除产品型号', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' }); }
  catch { return; }
  await apiRequest({ url: `/api/products/${encodeURIComponent(product.code)}`, method: 'DELETE' });
  ElMessage.success('产品型号已删除');
  await load();
}

onMounted(load);
</script>

<template>
  <div class="product-page">
    <div class="page-actions">
      <div><h1 class="page-title">产品库</h1><p class="page-description">统一维护网关型号及硬件规格，作为出厂登记的型号来源</p></div>
      <el-button type="primary" :icon="Plus" @click="open()">新增产品型号</el-button>
    </div>
    <div class="product-summary">
      <div><span>产品型号</span><strong>{{ products.length }}</strong></div>
      <div><span>可登记型号</span><strong>{{ enabledCount }}</strong></div>
      <div><span>规格待确认</span><strong>{{ unknownCount }}</strong></div>
    </div>
    <section class="table-panel">
      <div class="product-toolbar">
        <el-input v-model="keyword" clearable placeholder="搜索型号或产品名称" aria-label="搜索产品" :prefix-icon="Search" />
        <el-button :icon="RefreshRight" :loading="loading" @click="load">刷新</el-button>
      </div>
      <el-table v-loading="loading" :data="filtered" empty-text="暂无产品型号，请新增产品后再登记出厂网关">
        <el-table-column label="型号 / 产品" min-width="205"><template #default="{ row }"><div class="product-name"><strong>{{ row.code }}</strong><span>{{ row.name }}</span><span v-if="row.serialPortCount != null && (row.rs232PortCount == null || row.rs485PortCount == null)">历史串口总数 {{ row.serialPortCount }}，接口类型待确认</span></div></template></el-table-column>
        <el-table-column label="网口" width="100"><template #default="{ row }">{{ row.ethernetPortCount === null ? '待确认' : `${row.ethernetPortCount} 个` }}</template></el-table-column>
        <el-table-column label="RS232" width="100"><template #default="{ row }">{{ row.rs232PortCount == null ? '待确认' : `${row.rs232PortCount} 路` }}</template></el-table-column>
        <el-table-column label="RS485" width="100"><template #default="{ row }">{{ row.rs485PortCount == null ? '待确认' : `${row.rs485PortCount} 路` }}</template></el-table-column>
        <el-table-column label="CAN" width="100"><template #default="{ row }">{{ row.canPortCount == null ? '待确认' : `${row.canPortCount} 路` }}</template></el-table-column>
        <el-table-column label="4G" width="100"><template #default="{ row }"><el-tag :type="row.supports4G === true ? 'success' : row.supports4G === false ? 'info' : 'warning'">{{ supportText(row.supports4G) }}</el-tag></template></el-table-column>
        <el-table-column label="WiFi" width="100"><template #default="{ row }"><el-tag :type="row.supportsWifi === true ? 'success' : row.supportsWifi === false ? 'info' : 'warning'">{{ supportText(row.supportsWifi) }}</el-tag></template></el-table-column>
        <el-table-column label="状态" width="100"><template #default="{ row }"><el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag></template></el-table-column>
        <el-table-column label="出厂网关" width="110"><template #default="{ row }">{{ row._count?.gateways || 0 }}</template></el-table-column>
        <el-table-column prop="description" label="规格说明" min-width="220" show-overflow-tooltip />
        <el-table-column label="操作" width="145" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="open(row)">编辑</el-button><el-button link type="danger" :disabled="!!row._count?.gateways" @click="remove(row)">删除</el-button></template></el-table-column>
      </el-table>
      <p class="product-note">停用后禁止新增出厂登记，已有网关不受影响。型号编码不可更改；规格变化不会自动修改网关的运行配置。</p>
    </section>
    <el-dialog v-model="dialogVisible" :title="editingCode ? '编辑产品型号' : '新增产品型号'" width="min(620px, 94vw)" :close-on-click-modal="!saving" :close-on-press-escape="!saving">
      <el-form label-position="top" class="product-form">
        <el-form-item label="型号编码"><el-input v-model="form.code" :disabled="!!editingCode || saving" maxlength="32" placeholder="例如 EG100，保存后不可更改" @input="form.code = form.code.toUpperCase()" /></el-form-item>
        <el-form-item label="产品名称"><el-input v-model="form.name" maxlength="64" placeholder="例如 EG100 边缘网关" :disabled="saving" /></el-form-item>
        <div class="product-form-grid">
          <el-form-item label="网口数量"><el-input-number v-model="form.ethernetPortCount" :min="0" :max="64" :precision="0" controls-position="right" placeholder="待确认" :disabled="saving" /></el-form-item>
          <el-form-item label="RS232 数量"><el-input-number v-model="form.rs232PortCount" :min="0" :max="64" :precision="0" controls-position="right" placeholder="待确认" :disabled="saving" /></el-form-item>
          <el-form-item label="RS485 数量"><el-input-number v-model="form.rs485PortCount" :min="0" :max="64" :precision="0" controls-position="right" placeholder="待确认" :disabled="saving" /></el-form-item>
          <el-form-item label="CAN 数量"><el-input-number v-model="form.canPortCount" :min="0" :max="64" :precision="0" controls-position="right" placeholder="待确认" :disabled="saving" /></el-form-item>
          <el-form-item label="4G 支持"><el-select v-model="form.supports4G" :disabled="saving"><el-option v-for="item in supportChoices" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item>
          <el-form-item label="WiFi 支持"><el-select v-model="form.supportsWifi" :disabled="saving"><el-option v-for="item in supportChoices" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item>
        </div>
        <el-form-item label="规格说明"><el-input v-model="form.description" type="textarea" :rows="2" maxlength="512" placeholder="接口类型、选配模块或其他规格说明" :disabled="saving" /></el-form-item>
        <el-form-item label="允许出厂登记"><el-switch v-model="form.enabled" active-text="启用" inactive-text="停用" :disabled="saving" /></el-form-item>
      </el-form>
      <template #footer><el-button :disabled="saving" @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.product-summary { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; margin-bottom: 20px; }
.product-summary > div { display: flex; align-items: center; justify-content: space-between; padding: 18px 22px; border: 1px solid #e3eaf4; border-radius: 12px; background: #fff; }
.product-summary span { color: #8290a8; font-size: 13px; }
.product-summary strong { color: #172b4d; font-size: 26px; }
.product-toolbar { display: flex; justify-content: space-between; gap: 16px; margin-bottom: 18px; }
.product-toolbar .el-input { max-width: 340px; }
.product-name { display: grid; gap: 4px; }
.product-name strong { color: #172b4d; }
.product-name span, .product-note { color: #8290a8; font-size: 12px; line-height: 1.7; }
.product-note { margin: 18px 0 0; }
.product-form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 18px; }
.product-form-grid .el-input-number { width: 100%; }
@media (max-width: 600px) { .product-summary { gap: 8px; } .product-summary > div { flex-direction: column; gap: 8px; padding: 12px 8px; } .product-form-grid { grid-template-columns: 1fr; } }
</style>
