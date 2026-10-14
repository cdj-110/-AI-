<script setup lang="ts">
import { reactive, ref, watch } from 'vue';
import { CircleCheck } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { apiRequest } from '../api/request';

interface BindVerification {
  sn: string;
  batchNo?: string;
  producedAt?: string;
  lastSeenAt?: string;
  online: boolean;
  suggestedName: string;
}

interface BoundDevice {
  id: string;
  name: string;
  deviceKey: string;
  status: string;
}

const visible = defineModel<boolean>('visible', { default: false });
const emit = defineEmits<{ bound: [device: BoundDevice] }>();
const step = ref(0);
const verifying = ref(false);
const binding = ref(false);
const verifiedGateway = ref<BindVerification | null>(null);
const boundDevice = ref<BoundDevice | null>(null);
const form = reactive({ sn: '', bindCode: '', name: '', location: '' });

function formatTime(value?: string) {
  return value ? new Date(value).toLocaleString() : '-';
}

function reset() {
  Object.assign(form, { sn: '', bindCode: '', name: '', location: '' });
  verifiedGateway.value = null;
  boundDevice.value = null;
  step.value = 0;
}

watch(visible, (opened) => {
  if (opened) reset();
});

async function verifyGateway() {
  const sn = form.sn.trim().toUpperCase();
  const bindCode = form.bindCode.trim();
  if (!sn || !bindCode) {
    ElMessage.warning('请填写 SN 和 6 位绑定码');
    return;
  }
  if (!/^\d{6}$/.test(bindCode)) {
    ElMessage.warning('绑定码必须是 6 位数字');
    return;
  }
  verifying.value = true;
  try {
    const result = await apiRequest<BindVerification>({
      url: '/api/factory-gateways/bind/verify',
      method: 'POST',
      data: { sn, bindCode },
    });
    verifiedGateway.value = result;
    form.sn = result.sn;
    if (!form.name.trim()) form.name = result.suggestedName;
    step.value = 1;
  } finally {
    verifying.value = false;
  }
}

async function bindGateway() {
  if (!verifiedGateway.value) return;
  await ElMessageBox.confirm(
    `确认将 ${verifiedGateway.value.sn} 绑定为“${form.name.trim() || verifiedGateway.value.suggestedName}”吗？`,
    '确认绑定网关',
    { confirmButtonText: '确认绑定', cancelButtonText: '返回检查', type: 'warning' },
  );
  binding.value = true;
  try {
    const result = await apiRequest<{ device: BoundDevice }>({
      url: '/api/factory-gateways/bind',
      method: 'POST',
      data: {
        sn: form.sn.trim(),
        bindCode: form.bindCode.trim(),
        name: form.name.trim() || undefined,
        location: form.location.trim() || undefined,
      },
    });
    boundDevice.value = result.device;
    step.value = 2;
    emit('bound', result.device);
    ElMessage.success('网关绑定成功，已加入当前租户');
  } finally {
    binding.value = false;
  }
}
</script>

<template>
  <el-dialog
    v-model="visible"
    title="新增网关"
    width="760px"
    class="gateway-bind-dialog"
    :close-on-click-modal="false"
    destroy-on-close
  >
    <el-steps :active="step" simple finish-status="success" class="bind-steps">
      <el-step title="校验凭证" />
      <el-step title="确认信息" />
      <el-step title="完成绑定" />
    </el-steps>

    <div v-if="step === 0" class="bind-stage">
      <h3>输入网关交付凭证</h3>
      <p class="stage-description">SN 和绑定码位于网关铭牌或设备交付单，校验不会修改当前配置。</p>
      <el-form label-position="top" @keyup.enter="verifyGateway">
        <el-form-item label="网关 SN">
          <el-input v-model="form.sn" maxlength="32" placeholder="例如 WK2026A1B2C3D4" />
        </el-form-item>
        <el-form-item label="6 位绑定码">
          <el-input v-model="form.bindCode" maxlength="6" inputmode="numeric" show-password placeholder="请输入 6 位数字" />
        </el-form-item>
      </el-form>
      <p class="help-text">绑定码不正确或已经绑定时，系统会给出明确提示，不会创建设备。</p>
    </div>

    <div v-else-if="step === 1 && verifiedGateway" class="bind-stage">
      <div class="verified-card">
        <span class="verified-icon"><el-icon><CircleCheck /></el-icon></span>
        <div><strong>交付凭证校验通过</strong><p>{{ verifiedGateway.sn }}</p></div>
        <el-tag :type="verifiedGateway.online ? 'success' : 'info'">{{ verifiedGateway.online ? '网关在线' : '当前离线' }}</el-tag>
      </div>
      <div class="gateway-summary">
        <div><span>生产批次</span><strong>{{ verifiedGateway.batchNo || '未记录' }}</strong></div>
        <div><span>出厂时间</span><strong>{{ formatTime(verifiedGateway.producedAt) }}</strong></div>
        <div><span>最近上线</span><strong>{{ formatTime(verifiedGateway.lastSeenAt) }}</strong></div>
      </div>
      <el-form label-position="top" class="device-form">
        <el-form-item label="网关名称">
          <el-input v-model="form.name" maxlength="64" placeholder="例如 1 号泵站网关" />
        </el-form-item>
        <el-form-item label="安装位置">
          <el-input v-model="form.location" maxlength="128" placeholder="例如 北区 1 号泵房，可选" />
        </el-form-item>
      </el-form>
    </div>

    <div v-else class="bind-stage success-stage">
      <el-icon class="success-icon"><CircleCheck /></el-icon>
      <h3>网关绑定成功</h3>
      <p>{{ boundDevice?.name }} 已加入当前租户，关闭弹框后会自动刷新设备列表。</p>
    </div>

    <template #footer>
      <template v-if="step === 0">
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" :loading="verifying" @click="verifyGateway">校验并继续</el-button>
      </template>
      <template v-else-if="step === 1">
        <el-button @click="step = 0; verifiedGateway = null">返回修改</el-button>
        <el-button type="primary" :loading="binding" @click="bindGateway">确认绑定</el-button>
      </template>
      <el-button v-else type="primary" @click="visible = false">完成</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.bind-steps { margin: -18px -20px 0; padding: 16px 24px; border-bottom: 1px solid #e8eef6; background: #f8fbff; }
.bind-stage { min-height: 280px; padding: 24px 10px 4px; }
.bind-stage h3 { margin: 0; color: #172033; font-size: 19px; }
.stage-description { margin: 7px 0 20px; color: #64748b; font-size: 13px; line-height: 1.65; }
.help-text { margin: 10px 0 0; padding-top: 14px; border-top: 1px solid #edf1f7; color: #8b96a8; font-size: 12px; }
.verified-card { display: flex; align-items: center; gap: 12px; padding: 15px; border: 1px solid #bbf7d0; border-radius: 13px; background: #f0fdf4; }
.verified-icon { display: grid; width: 36px; height: 36px; place-items: center; border-radius: 50%; background: #16a34a; color: #fff; font-size: 21px; }
.verified-card > div { flex: 1; }
.verified-card strong { color: #166534; }
.verified-card p { margin: 3px 0 0; color: #15803d; font-family: Consolas, monospace; }
.gateway-summary { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; margin: 14px 0 18px; }
.gateway-summary div { padding: 11px 12px; border: 1px solid #e2e8f0; border-radius: 10px; }
.gateway-summary span, .gateway-summary strong { display: block; }
.gateway-summary span { margin-bottom: 4px; color: #94a3b8; font-size: 12px; }
.gateway-summary strong { color: #334155; font-size: 13px; }
.success-stage { display: flex; flex-direction: column; align-items: center; justify-content: center; text-align: center; }
.success-icon { color: #16a34a; font-size: 56px; }
.success-stage h3 { margin-top: 12px; }
.success-stage p { color: #64748b; }
@media (max-width: 640px) {
  .gateway-summary { grid-template-columns: 1fr; }
  .bind-stage { min-height: 240px; padding-inline: 0; }
}
</style>
