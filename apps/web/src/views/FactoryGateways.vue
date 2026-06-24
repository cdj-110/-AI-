<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Download, RefreshRight, Search } from '@element-plus/icons-vue';
import { apiRequest } from '../api/request';
import { useUserStore } from '../stores/user';

interface FactoryGatewayItem {
  id: string;
  hardwareId: string;
  sn: string;
  status: string;
  batchNo?: string;
  producedAt?: string;
  firstSeenAt?: string;
  lastSeenAt?: string;
  boundAt?: string;
  device?: {
    id: string;
    name: string;
    deviceKey: string;
    tenantId: string;
    status: string;
  };
}

interface ActivationFile {
  enabled: boolean;
  hardwareId: string;
  sn: string;
  deviceSecret: string;
  broker: string;
}

interface RegisterResult extends FactoryGatewayItem {
  deviceSecret: string;
  bindCode: string;
  activationFile: ActivationFile;
}

interface ResetBindCodeResult extends FactoryGatewayItem {
  bindCode: string;
}

interface ActivationResetResult extends FactoryGatewayItem {
  deviceSecret: string;
  activationFile: ActivationFile;
}

const userStore = useUserStore();
const loading = ref(false);
const registering = ref(false);
const binding = ref(false);
const resettingId = ref('');
const resettingActivationId = ref('');
const gateways = ref<FactoryGatewayItem[]>([]);
const total = ref(0);
const latestResult = ref<RegisterResult | null>(null);
const latestResetResult = ref<ResetBindCodeResult | null>(null);
const latestActivationResetResult = ref<ActivationResetResult | null>(null);
const query = reactive({ page: 1, pageSize: 10, keyword: '' });
const registerForm = reactive({ hardwareId: '', batchNo: '' });
const bindForm = reactive({ sn: '', bindCode: '', name: '', location: '' });

const isSuperAdmin = computed(() => userStore.userInfo?.role === 'SUPER_ADMIN');
const activationText = computed(() => JSON.stringify(latestResult.value?.activationFile ?? {}, null, 2));
const resetActivationText = computed(() => JSON.stringify(latestActivationResetResult.value?.activationFile ?? {}, null, 2));

function statusText(status: string) {
  return { UNBOUND: '未绑定', BOUND: '已绑定', DISABLED: '已禁用' }[status] ?? status;
}

function statusTagType(status: string) {
  return status === 'BOUND' ? 'success' : status === 'DISABLED' ? 'danger' : 'warning';
}

function formatTime(value?: string) {
  return value ? new Date(value).toLocaleString() : '-';
}

async function loadGateways() {
  if (!isSuperAdmin.value) return;
  loading.value = true;
  try {
    const data = await apiRequest<{ items: FactoryGatewayItem[]; total: number }>({
      url: '/api/factory-gateways',
      method: 'GET',
      params: query,
    });
    gateways.value = data.items;
    total.value = data.total;
  } finally {
    loading.value = false;
  }
}

async function searchGateways() {
  query.page = 1;
  await loadGateways();
}

async function registerGateway() {
  const hardwareId = registerForm.hardwareId.trim().toUpperCase();
  if (!hardwareId) {
    ElMessage.warning('请先填写 HardwareId');
    return;
  }
  if (!/^[0-9A-F]{32}$/.test(hardwareId)) {
    ElMessage.warning('请填写网关设备激活页显示的完整 32 位 HardwareId');
    return;
  }
  registering.value = true;
  try {
    latestResult.value = await apiRequest<RegisterResult>({
      url: '/api/factory-gateways',
      method: 'POST',
      data: {
        hardwareId,
        batchNo: registerForm.batchNo.trim() || undefined,
      },
    });
    registerForm.hardwareId = '';
    ElMessage.success('出厂网关登记成功，请保存一次性凭证');
    await loadGateways();
  } finally {
    registering.value = false;
  }
}

async function bindGateway() {
  if (!bindForm.sn.trim() || !bindForm.bindCode.trim()) {
    ElMessage.warning('请填写 SN 和绑定码');
    return;
  }
  binding.value = true;
  try {
    await apiRequest({
      url: '/api/factory-gateways/bind',
      method: 'POST',
      data: {
        sn: bindForm.sn.trim(),
        bindCode: bindForm.bindCode.trim(),
        name: bindForm.name.trim() || undefined,
        location: bindForm.location.trim() || undefined,
      },
    });
    Object.assign(bindForm, { sn: '', bindCode: '', name: '', location: '' });
    ElMessage.success('网关绑定成功，已加入设备管理');
    await loadGateways();
  } finally {
    binding.value = false;
  }
}

async function resetBindCode(row: FactoryGatewayItem) {
  await ElMessageBox.confirm(
    `确认重置 ${row.sn} 的绑定码吗？新绑定码只会展示一次，请重置后立即保存。`,
    '重置绑定码',
    { confirmButtonText: '确认重置', cancelButtonText: '取消', type: 'warning' },
  );
  resettingId.value = row.id;
  try {
    latestResetResult.value = await apiRequest<ResetBindCodeResult>({
      url: `/api/factory-gateways/${row.id}/bind-code/reset`,
      method: 'POST',
    });
    ElMessage.success('绑定码已重置，请立即保存新绑定码');
    await loadGateways();
  } finally {
    resettingId.value = '';
  }
}

async function resetActivation(row: FactoryGatewayItem) {
  await ElMessageBox.confirm(
    `确认重新生成 ${row.sn} 的激活文件吗？旧激活文件将立即失效，必须把新文件重新导入网关。`,
    '重新生成激活文件',
    { confirmButtonText: '确认重新生成', cancelButtonText: '取消', type: 'warning' },
  );
  resettingActivationId.value = row.id;
  try {
    latestActivationResetResult.value = await apiRequest<ActivationResetResult>({
      url: `/api/factory-gateways/${row.id}/activation/reset`,
      method: 'POST',
    });
    ElMessage.success('新激活文件已生成，请立即下载并重新导入网关');
  } finally {
    resettingActivationId.value = '';
  }
}

function fillBindForm(sn: string, bindCode: string) {
  bindForm.sn = sn;
  bindForm.bindCode = bindCode;
  ElMessage.success('已填入绑定表单');
}

async function copyText(text: string, label: string) {
  await navigator.clipboard.writeText(text);
  ElMessage.success(`${label} 已复制`);
}

function downloadActivationFile() {
  if (!latestResult.value) return;
  const blob = new Blob([activationText.value], { type: 'application/json;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `activation-${latestResult.value.sn}.json`;
  link.click();
  URL.revokeObjectURL(url);
}

function downloadResetActivationFile() {
  if (!latestActivationResetResult.value) return;
  const blob = new Blob([resetActivationText.value], { type: 'application/json;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `activation-${latestActivationResetResult.value.sn}.json`;
  link.click();
  URL.revokeObjectURL(url);
}

onMounted(loadGateways);
</script>

<template>
  <div>
    <div class="page-actions">
      <div>
        <h1 class="page-title">出厂网关</h1>
        <p class="page-description">管理 HardwareId、SN、DeviceSecret 和绑定码，让网关出厂后可以自动接入云平台。</p>
      </div>
      <el-button v-if="isSuperAdmin" :icon="RefreshRight" @click="loadGateways">刷新</el-button>
    </div>

    <section class="flow-panel">
      <div class="flow-step">
        <strong>1. 工厂登记</strong>
        <span>录入硬件 HardwareId，平台生成 SN、DeviceSecret、BindCode。</span>
      </div>
      <div class="flow-step">
        <strong>2. 烧录激活</strong>
        <span>工厂只写入 DeviceSecret 或激活文件，网关开机读取硬件 ID 自动连云。</span>
      </div>
      <div class="flow-step">
        <strong>3. 客户绑定</strong>
        <span>客户输入 SN + BindCode，平台把在线网关绑定到当前租户。</span>
      </div>
    </section>

    <div class="factory-grid">
      <section v-if="isSuperAdmin" class="table-panel factory-card">
        <div class="section-title">
          <h2>登记出厂网关</h2>
          <span>仅管理员可生成一次性凭证</span>
        </div>
        <el-form label-position="top">
          <el-form-item label="HardwareId">
            <el-input v-model="registerForm.hardwareId" placeholder="扫码或粘贴硬件唯一 ID" />
          </el-form-item>
          <el-form-item label="生产批次">
            <el-input v-model="registerForm.batchNo" placeholder="例如 202606-BJ-01，可选" />
          </el-form-item>
          <el-button type="primary" :loading="registering" @click="registerGateway">生成 SN 与激活文件</el-button>
        </el-form>
      </section>

      <section class="table-panel factory-card">
        <div class="section-title">
          <h2>客户绑定网关</h2>
          <span>输入设备铭牌或交付单上的 SN 和绑定码</span>
        </div>
        <el-form label-position="top">
          <el-form-item label="SN">
            <el-input v-model="bindForm.sn" placeholder="例如 WK2026A1B2C3D4" />
          </el-form-item>
          <el-form-item label="绑定码">
            <el-input v-model="bindForm.bindCode" placeholder="6 位绑定码" />
          </el-form-item>
          <el-form-item label="网关名称">
            <el-input v-model="bindForm.name" placeholder="可选，默认使用 SN" />
          </el-form-item>
          <el-form-item label="安装位置">
            <el-input v-model="bindForm.location" placeholder="可选，例如 1 号车间" />
          </el-form-item>
          <el-button type="primary" :loading="binding" @click="bindGateway">绑定到当前账号</el-button>
        </el-form>
      </section>
    </div>

    <section v-if="latestResult" class="table-panel credential-panel">
      <div class="section-title">
        <h2>本次生成的交付信息</h2>
        <span>DeviceSecret 和 BindCode 只在本次生成后展示，请立即保存</span>
      </div>
      <div class="credential-grid">
        <div>
          <label>SN</label>
          <div class="copy-line">
            <code>{{ latestResult.sn }}</code>
            <el-button text @click="copyText(latestResult.sn, 'SN')">复制</el-button>
          </div>
        </div>
        <div>
          <label>绑定码</label>
          <div class="copy-line">
            <code>{{ latestResult.bindCode }}</code>
            <el-button text @click="copyText(latestResult.bindCode, '绑定码')">复制</el-button>
          </div>
        </div>
        <div>
          <label>DeviceSecret</label>
          <div class="copy-line">
            <code>{{ latestResult.deviceSecret }}</code>
            <el-button text @click="copyText(latestResult.deviceSecret, 'DeviceSecret')">复制</el-button>
          </div>
        </div>
      </div>
      <div class="activation-box">
        <div class="activation-header">
          <strong>activation.local.json</strong>
          <el-button :icon="Download" @click="downloadActivationFile">下载激活文件</el-button>
        </div>
        <pre>{{ activationText }}</pre>
      </div>
    </section>

    <section v-if="latestResetResult" class="table-panel credential-panel">
      <div class="section-title">
        <h2>新绑定码</h2>
        <span>绑定码只在重置后展示一次，请立即复制给客户或写入交付单</span>
      </div>
      <div class="credential-grid reset-grid">
        <div>
          <label>SN</label>
          <div class="copy-line">
            <code>{{ latestResetResult.sn }}</code>
            <el-button text @click="copyText(latestResetResult.sn, 'SN')">复制</el-button>
          </div>
        </div>
        <div>
          <label>新绑定码</label>
          <div class="copy-line">
            <code>{{ latestResetResult.bindCode }}</code>
            <el-button text @click="copyText(latestResetResult.bindCode, '新绑定码')">复制</el-button>
          </div>
        </div>
        <div class="reset-actions">
          <el-button type="primary" @click="fillBindForm(latestResetResult.sn, latestResetResult.bindCode)">填入绑定表单</el-button>
        </div>
      </div>
    </section>

    <section v-if="latestActivationResetResult" class="table-panel credential-panel">
      <div class="section-title">
        <h2>新激活文件</h2>
        <span>DeviceSecret 已轮换，旧激活文件已失效，请立即下载并重新导入网关</span>
      </div>
      <div class="activation-box">
        <div class="activation-header">
          <strong>activation.local.json</strong>
          <el-button type="primary" :icon="Download" @click="downloadResetActivationFile">下载新激活文件</el-button>
        </div>
        <pre>{{ resetActivationText }}</pre>
      </div>
    </section>

    <section v-if="isSuperAdmin" class="table-panel">
      <div class="table-toolbar">
        <el-input v-model="query.keyword" clearable placeholder="搜索 HardwareId 或 SN" style="max-width: 320px" @keyup.enter="searchGateways">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-button @click="searchGateways">查询</el-button>
      </div>

      <el-table v-loading="loading" :data="gateways" empty-text="暂无出厂网关">
        <el-table-column prop="sn" label="SN" min-width="150" />
        <el-table-column prop="hardwareId" label="HardwareId" min-width="190" show-overflow-tooltip />
        <el-table-column label="状态" width="100">
          <template #default="{ row }"><el-tag :type="statusTagType(row.status)">{{ statusText(row.status) }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="batchNo" label="批次" min-width="130" />
        <el-table-column label="绑定设备" min-width="170">
          <template #default="{ row }">
            <router-link v-if="row.device" :to="`/devices/${row.device.id}`">{{ row.device.name }}</router-link>
            <span v-else class="muted">未绑定</span>
          </template>
        </el-table-column>
        <el-table-column label="最近心跳" min-width="170">
          <template #default="{ row }">{{ formatTime(row.lastSeenAt) }}</template>
        </el-table-column>
        <el-table-column label="绑定时间" min-width="170">
          <template #default="{ row }">{{ formatTime(row.boundAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="250" fixed="right">
          <template #default="{ row }">
            <el-button
              text
              type="primary"
              :disabled="row.status === 'BOUND'"
              :loading="resettingId === row.id"
              @click="resetBindCode(row)"
            >
              重置绑定码
            </el-button>
            <el-button
              text
              type="warning"
              :disabled="row.status === 'BOUND'"
              :loading="resettingActivationId === row.id"
              @click="resetActivation(row)"
            >
              重新生成激活文件
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination">
        <el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" layout="total, prev, pager, next" :total="total" @current-change="loadGateways" />
      </div>
    </section>
  </div>
</template>

<style scoped>
.flow-panel { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; margin-bottom: 18px; }
.flow-step { padding: 16px; border: 1px solid #dbeafe; border-radius: 14px; background: linear-gradient(135deg, #fff, #eff6ff); }
.flow-step strong { display: block; color: #172033; font-size: 15px; }
.flow-step span { display: block; margin-top: 8px; color: #64748b; font-size: 13px; line-height: 1.6; }
.factory-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; margin-bottom: 18px; }
.factory-card { min-height: 100%; }
.section-title { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 16px; }
.section-title h2 { margin: 0; color: #172033; font-size: 18px; }
.section-title span { color: #8b96a8; font-size: 13px; }
.credential-panel { margin-bottom: 18px; border-color: #bfdbfe; background: #f8fbff; }
.credential-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.credential-grid label { display: block; margin-bottom: 8px; color: #64748b; font-size: 13px; }
.reset-actions { display: flex; align-items: flex-end; }
.reset-actions .el-button { width: 100%; }
.copy-line { display: flex; min-width: 0; align-items: center; justify-content: space-between; gap: 10px; padding: 10px 12px; border: 1px solid #e2e8f0; border-radius: 10px; background: #fff; }
.copy-line code { overflow: hidden; color: #172033; text-overflow: ellipsis; white-space: nowrap; }
.activation-box { margin-top: 16px; border: 1px solid #e2e8f0; border-radius: 12px; background: #fff; overflow: hidden; }
.activation-header { display: flex; align-items: center; justify-content: space-between; padding: 12px 14px; border-bottom: 1px solid #e2e8f0; }
.activation-box pre { margin: 0; padding: 14px; overflow: auto; color: #334155; font-size: 13px; line-height: 1.7; }
.muted { color: #94a3b8; }
@media (max-width: 1000px) {
  .flow-panel, .factory-grid, .credential-grid { grid-template-columns: 1fr; }
}
</style>
