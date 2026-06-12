<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { Plus, Search } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus';
import { apiRequest } from '../api/request';
import { useUserStore } from '../stores/user';

interface TenantItem {
  id: string;
  name: string;
}

interface TemplateMetric {
  id?: string;
  identifier: string;
  name: string;
  dataType: string;
  unit?: string;
  decimals: number;
  accessMode: string;
  enabled: boolean;
  sortOrder: number;
}

interface ModelTemplateItem {
  id: string;
  tenantId: string;
  tenant?: TenantItem;
  name: string;
  description?: string;
  deviceType: string;
  metrics?: TemplateMetric[];
  _count?: { metrics: number };
  createdAt: string;
}

interface SourceDeviceOption {
  id: string;
  name: string;
  deviceKey: string;
  _count: { metrics: number };
}

const userStore = useUserStore();
const loading = ref(false);
const dialogVisible = ref(false);
const fromDeviceDialogVisible = ref(false);
const metricsDrawerVisible = ref(false);
const editingId = ref('');
const currentTemplate = ref<ModelTemplateItem>();
const templates = ref<ModelTemplateItem[]>([]);
const sourceDevices = ref<SourceDeviceOption[]>([]);
const templateMetrics = ref<TemplateMetric[]>([]);
const formRef = ref<FormInstance>();
const query = reactive({ keyword: '' });
const form = reactive({ name: '', description: '', deviceType: 'DIRECT' });
const fromDeviceForm = reactive({ deviceId: '', name: '', description: '' });
const canManage = computed(() => userStore.userInfo?.role !== 'TENANT_USER');
const isSuperAdmin = computed(() => userStore.userInfo?.role === 'SUPER_ADMIN');

const rules: FormRules = {
  name: [{ required: true, message: '请输入模板名称', trigger: 'blur' }],
  deviceType: [{ required: true, message: '请选择适用设备类型', trigger: 'change' }],
};

async function loadTemplates() {
  loading.value = true;
  try {
    templates.value = await apiRequest<ModelTemplateItem[]>({ url: '/api/model-templates', method: 'GET', params: query });
  } finally {
    loading.value = false;
  }
}

function resetForm() {
  Object.assign(form, { name: '', description: '', deviceType: 'DIRECT' });
}

function openCreate() {
  editingId.value = '';
  resetForm();
  dialogVisible.value = true;
}

async function openCreateFromDevice() {
  Object.assign(fromDeviceForm, { deviceId: '', name: '', description: '' });
  sourceDevices.value = await apiRequest<SourceDeviceOption[]>({ url: '/api/devices/model-templates', method: 'GET' });
  fromDeviceDialogVisible.value = true;
}

function openEdit(template: ModelTemplateItem) {
  editingId.value = template.id;
  Object.assign(form, {
    name: template.name,
    description: template.description ?? '',
    deviceType: template.deviceType,
  });
  dialogVisible.value = true;
}

async function submit() {
  if (!(await formRef.value?.validate())) return;
  await apiRequest({
    url: editingId.value ? `/api/model-templates/${editingId.value}` : '/api/model-templates',
    method: editingId.value ? 'PATCH' : 'POST',
    data: { ...form, description: form.description || undefined },
  });
  ElMessage.success(editingId.value ? '物模型模板已更新' : '物模型模板已创建');
  dialogVisible.value = false;
  await loadTemplates();
}

async function submitCreateFromDevice() {
  if (!fromDeviceForm.deviceId) {
    ElMessage.warning('请选择来源设备');
    return;
  }
  await apiRequest({
    url: '/api/model-templates/from-device',
    method: 'POST',
    data: {
      deviceId: fromDeviceForm.deviceId,
      name: fromDeviceForm.name || undefined,
      description: fromDeviceForm.description || undefined,
    },
  });
  ElMessage.success('已从设备生成物模型模板');
  fromDeviceDialogVisible.value = false;
  await loadTemplates();
}

async function duplicate(template: ModelTemplateItem) {
  await apiRequest({ url: `/api/model-templates/${template.id}/duplicate`, method: 'POST' });
  ElMessage.success('物模型模板已复制');
  await loadTemplates();
}

async function remove(template: ModelTemplateItem) {
  await ElMessageBox.confirm(`确定删除物模型模板“${template.name}”吗？`, '删除确认', { type: 'warning' });
  await apiRequest({ url: `/api/model-templates/${template.id}`, method: 'DELETE' });
  ElMessage.success('物模型模板已删除');
  await loadTemplates();
}

async function openMetrics(template: ModelTemplateItem) {
  // 字段抽屉编辑的是模板字段，保存后只影响后续复用，不会自动改已创建的设备。
  const detail = await apiRequest<ModelTemplateItem>({ url: `/api/model-templates/${template.id}`, method: 'GET' });
  currentTemplate.value = detail;
  templateMetrics.value = (detail.metrics ?? []).map((metric) => ({ ...metric }));
  metricsDrawerVisible.value = true;
}

function addMetric() {
  // 手动新增字段用于提前定义物模型；设备上报后也会自动发现设备自己的字段。
  templateMetrics.value.push({
    identifier: '',
    name: '',
    dataType: 'NUMBER',
    unit: '',
    decimals: 2,
    accessMode: 'READ_ONLY',
    enabled: true,
    sortOrder: (templateMetrics.value.length + 1) * 10,
  });
}

function removeMetric(index: number) {
  templateMetrics.value.splice(index, 1);
}

async function saveMetrics() {
  // 后端采用整表替换保存模板字段，所以前端删除行后保存即可完成删除。
  if (!currentTemplate.value) return;
  const invalid = templateMetrics.value.find((metric) => !metric.identifier.trim() || !metric.name.trim());
  if (invalid) {
    ElMessage.warning('请补全字段标识符和显示名称');
    return;
  }
  await apiRequest({
    url: `/api/model-templates/${currentTemplate.value.id}/metrics`,
    method: 'PUT',
    data: { metrics: templateMetrics.value },
  });
  ElMessage.success('物模型字段已保存');
  await loadTemplates();
}

function deviceTypeLabel(type: string) {
  return { GATEWAY: '网关', GATEWAY_CHILD: '子设备', DIRECT: '直连' }[type] ?? type;
}

function deviceTypeTagType(_type: string) {
  return '';
}

function deviceTypeTagClass(type: string) {
  return {
    GATEWAY: 'gateway-device-tag',
    GATEWAY_CHILD: 'gateway-child-tag',
    DIRECT: 'direct-device-tag',
  }[type] ?? '';
}

onMounted(loadTemplates);
</script>

<template>
  <div>
    <div class="page-actions">
      <div>
        <h1 class="page-title">物模型管理</h1>
        <p class="page-description">维护可复用的设备字段模板，新建设备时可直接套用。</p>
      </div>
      <div v-if="canManage" class="page-action-buttons">
        <el-button @click="openCreateFromDevice">从设备生成</el-button>
        <el-button type="primary" :icon="Plus" @click="openCreate">新增模板</el-button>
      </div>
    </div>

    <section class="table-panel">
      <div class="table-toolbar">
        <el-input v-model="query.keyword" clearable placeholder="搜索模板名称" style="max-width: 300px" @keyup.enter="loadTemplates">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-button @click="loadTemplates">查询</el-button>
      </div>
      <el-table v-loading="loading" :data="templates">
        <el-table-column prop="name" label="模板名称" min-width="170" />
        <el-table-column label="适用类型" width="120"><template #default="{ row }"><el-tag class="device-type-tag" :class="deviceTypeTagClass(row.deviceType)" :type="deviceTypeTagType(row.deviceType)">{{ deviceTypeLabel(row.deviceType) }}</el-tag></template></el-table-column>
        <el-table-column label="字段数" width="90"><template #default="{ row }">{{ row._count?.metrics ?? 0 }}</template></el-table-column>
        <el-table-column prop="description" label="说明" min-width="220" show-overflow-tooltip />
        <el-table-column v-if="isSuperAdmin" prop="tenant.name" label="所属租户" min-width="160" />
        <el-table-column label="创建时间" min-width="180"><template #default="{ row }">{{ new Date(row.createdAt).toLocaleString() }}</template></el-table-column>
        <el-table-column label="操作" width="260" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openMetrics(row)">字段</el-button>
            <el-button v-if="canManage" link type="primary" @click="duplicate(row)">复制</el-button>
            <el-button v-if="canManage" link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button v-if="canManage" link type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </section>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑物模型模板' : '新增物模型模板'" width="520px">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="110px">
        <el-form-item label="模板名称" prop="name"><el-input v-model="form.name" placeholder="例如 温湿度传感器" /></el-form-item>
        <el-form-item label="适用类型" prop="deviceType">
          <el-select v-model="form.deviceType" class="full-width">
            <el-option label="直连设备" value="DIRECT" />
            <el-option label="网关" value="GATEWAY" />
            <el-option label="网关子设备" value="GATEWAY_CHILD" />
          </el-select>
        </el-form-item>
        <el-form-item label="说明"><el-input v-model="form.description" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" @click="submit">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="fromDeviceDialogVisible" title="从设备生成物模型模板" width="540px">
      <el-form :model="fromDeviceForm" label-width="100px">
        <el-form-item label="来源设备">
          <el-select v-model="fromDeviceForm.deviceId" filterable class="full-width" placeholder="选择已有物模型字段的设备">
            <el-option v-for="device in sourceDevices" :key="device.id" :label="`${device.name} (${device.deviceKey}) · ${device._count.metrics} 个字段`" :value="device.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="模板名称"><el-input v-model="fromDeviceForm.name" placeholder="留空则使用设备名称自动生成" /></el-form-item>
        <el-form-item label="说明"><el-input v-model="fromDeviceForm.description" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="fromDeviceDialogVisible = false">取消</el-button><el-button type="primary" @click="submitCreateFromDevice">生成模板</el-button></template>
    </el-dialog>

    <el-drawer v-model="metricsDrawerVisible" :title="`${currentTemplate?.name ?? ''} · 字段配置`" size="min(1080px, 96vw)">
      <div class="drawer-heading">
        <div>
          <h3>物模型字段</h3>
          <p>字段会在新建设备选择该模板时复制到设备物模型中。</p>
        </div>
        <div v-if="canManage" class="drawer-actions">
          <el-button :icon="Plus" @click="addMetric">新增字段</el-button>
          <el-button type="primary" @click="saveMetrics">保存字段</el-button>
        </div>
      </div>
      <el-table :data="templateMetrics" size="small" empty-text="暂无字段">
        <template #empty>
          <div class="metric-empty">
            <p>暂无字段</p>
            <el-button v-if="canManage" type="primary" plain :icon="Plus" @click="addMetric">新增字段</el-button>
          </div>
        </template>
        <el-table-column label="标识符" min-width="140"><template #default="{ row }"><el-input v-model="row.identifier" :disabled="!canManage" /></template></el-table-column>
        <el-table-column label="显示名称" min-width="140"><template #default="{ row }"><el-input v-model="row.name" :disabled="!canManage" /></template></el-table-column>
        <el-table-column label="类型" width="110"><template #default="{ row }"><el-select v-model="row.dataType" :disabled="!canManage"><el-option label="数字" value="NUMBER" /><el-option label="文本" value="STRING" /><el-option label="布尔" value="BOOLEAN" /><el-option label="对象" value="OBJECT" /></el-select></template></el-table-column>
        <el-table-column label="单位" width="90"><template #default="{ row }"><el-input v-model="row.unit" :disabled="!canManage" /></template></el-table-column>
        <el-table-column label="权限" width="110"><template #default="{ row }"><el-select v-model="row.accessMode" :disabled="!canManage"><el-option label="只读" value="READ_ONLY" /><el-option label="读写" value="READ_WRITE" /></el-select></template></el-table-column>
        <el-table-column label="排序" width="95"><template #default="{ row }"><el-input-number v-model="row.sortOrder" class="metric-number" :min="0" controls-position="right" :disabled="!canManage" /></template></el-table-column>
        <el-table-column label="小数位" width="95"><template #default="{ row }"><el-input-number v-model="row.decimals" class="metric-number" :min="0" :max="6" controls-position="right" :disabled="!canManage" /></template></el-table-column>
        <el-table-column label="展示" width="70"><template #default="{ row }"><el-switch v-model="row.enabled" :disabled="!canManage" /></template></el-table-column>
        <el-table-column v-if="canManage" label="操作" width="70"><template #default="{ $index }"><el-button link type="danger" @click="removeMetric($index)">删除</el-button></template></el-table-column>
      </el-table>
    </el-drawer>
  </div>
</template>

<style scoped>
.page-action-buttons { display: flex; gap: 10px; }
.drawer-heading { display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px; }
.drawer-heading h3 { margin: 0; color: #111827; font-size: 16px; }
.drawer-heading p { margin: 6px 0 0; color: #94a3b8; font-size: 13px; }
.drawer-actions { display: flex; gap: 10px; }
.metric-empty { display: grid; gap: 10px; place-items: center; padding: 22px 0; color: #94a3b8; }
.metric-empty p { margin: 0; }
.metric-number { width: 100%; }
.device-type-tag.gateway-device-tag { --el-tag-bg-color: #e6f4ff; --el-tag-border-color: #91caff; --el-tag-text-color: #1677ff; }
.device-type-tag.gateway-child-tag { --el-tag-bg-color: #fff7e6; --el-tag-border-color: #ffd591; --el-tag-text-color: #fa8c16; }
.device-type-tag.direct-device-tag { --el-tag-bg-color: #f6ffed; --el-tag-border-color: #b7eb8f; --el-tag-text-color: #52c41a; }
</style>
