<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { apiRequest } from '../api/request';

const props = defineProps<{ deviceId: string; deviceName: string }>();
interface Point { identifier: string; name: string; dataType: string; unit?: string }
interface Capabilities { points: Point[]; available: boolean; reason: string }
const emit = defineEmits<{ capabilities: [value: Capabilities]; pending: [metrics: string[]] }>();
interface Task { id: string; requestId: string; metric: string; value: unknown; status: string; message?: string; createdAt: string }
const visible = ref(false), historyVisible = ref(false), loading = ref(false), historyLoading = ref(false), sending = ref(false);
const points = ref<Point[]>([]), available = ref(false), reason = ref(''), allTasks = ref<Task[]>([]);
const selected = ref(''), input = ref(''), boolValue = ref(false), boolTouched = ref(false), requestId = ref('');
const submitted = ref(false), task = ref<Task>();
const point = computed(() => points.value.find(item => item.identifier === selected.value));
const recentTasks = computed(() => allTasks.value.filter(item => item.metric === selected.value).slice(0, 3));
const names: Record<string, string> = { SENT: '等待网关回执', SUCCEEDED: '执行成功', FAILED: '执行失败', TIMEOUT: '结果未确认' };
const notified = new Set<string>();
const busyBoolean = new Set<string>();
const booleanRequests = new Map<string, { requestId: string; startedAt: number }>();
function setBooleanBusy(metric: string, busy: boolean) {
  if (busy) busyBoolean.add(metric); else busyBoolean.delete(metric);
  emit('pending', [...busyBoolean]);
}
function reconcileBooleanRequests(history: Task[]) {
  for (const [metric, pending] of booleanRequests) {
    const result = history.find(item => item.requestId === pending.requestId);
    if (result && result.status !== 'SENT') {
      notify(result); booleanRequests.delete(metric); setBooleanBusy(metric, false);
    } else if (Date.now() - pending.startedAt > 45000) {
      ElMessage.warning('下发结果仍未确认，请查看下发记录后再操作');
      booleanRequests.delete(metric); setBooleanBusy(metric, false);
    }
  }
}
function newRequestId() {
  // getRandomValues also works on LAN HTTP, where randomUUID is unavailable.
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  bytes[6] = (bytes[6] & 15) | 64; bytes[8] = (bytes[8] & 63) | 128;
  const hex = [...bytes].map(value => value.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}
let timer: ReturnType<typeof setTimeout> | undefined;
let epoch = 0;
let disposed = false;
function stop() { if (timer) clearTimeout(timer); timer = undefined; }
function notify(result: Task) {
  if (result.status === 'SENT' || notified.has(result.id + result.status)) return;
  notified.add(result.id + result.status);
  const message = result.message || names[result.status];
  if (result.status === 'SUCCEEDED') ElMessage.success(message);
  else if (result.status === 'FAILED') ElMessage.error(message);
  else ElMessage.warning(message);
}
async function refresh(silent = false) {
  const id = props.deviceId, generation = epoch;
  if (historyVisible.value && !silent) historyLoading.value = true;
  try {
    const [caps, history] = await Promise.all([
      apiRequest<Capabilities>({ url: `/api/devices/${id}/commands/capabilities`, method: 'GET', silentError: silent }),
      visible.value || historyVisible.value || booleanRequests.size ? apiRequest<Task[]>({ url: `/api/devices/${id}/commands`, method: 'GET', silentError: silent }) : Promise.resolve([]),
    ]);
    if (generation !== epoch || id !== props.deviceId) return;
    points.value = Array.isArray(caps.points) ? caps.points : []; available.value = !!caps.available; reason.value = caps.reason || ''; if (visible.value || historyVisible.value) allTasks.value = history;
    emit('capabilities', { points: points.value, available: available.value, reason: reason.value });
    if (visible.value && !point.value) reason.value = '该点位当前不可写，请检查网关点位配置';
    if (requestId.value) {
      const result = history.find(item => item.requestId === requestId.value);
      if (result) { task.value = result; notify(result); }
    }
    reconcileBooleanRequests(history);
  } catch { if (generation === epoch) { available.value = false; reason.value = '暂时无法查询点位能力或结果，请检查网络；请勿重复下发'; emit('capabilities', { points: points.value, available: false, reason: reason.value }); reconcileBooleanRequests([]); } }
  finally { historyLoading.value = false; }
}
function poll() {
  stop();
  if (disposed) return;
  const generation = epoch;
  timer = setTimeout(async () => { await refresh(true); if (generation === epoch && !disposed) poll(); }, visible.value || booleanRequests.size ? 2000 : historyVisible.value ? 5000 : 15000);
}
function showHistory() {
  visible.value = false;
  historyVisible.value = true;
}
async function open(metric: string) {
  if (sending.value || !metric) return;
  stop();
  historyVisible.value = false;
  visible.value = true; loading.value = true; epoch++;
  selected.value = metric || ''; input.value = ''; boolValue.value = false; boolTouched.value = false;
  points.value = []; available.value = false; reason.value = ''; allTasks.value = [];
  requestId.value = ''; submitted.value = false; task.value = undefined;
  try { await refresh(); }
  finally { loading.value = false; poll(); }
}
async function send() {
  if (sending.value || submitted.value || !point.value || !available.value) return;
  if (point.value.dataType === 'BOOLEAN' && !boolTouched.value) { ElMessage.warning('请先切换开关'); return; }
  const value = point.value.dataType === 'BOOLEAN' ? boolValue.value : point.value.dataType === 'NUMBER' ? Number(input.value) : input.value;
  if (point.value.dataType === 'NUMBER' && (!input.value.trim() || !Number.isFinite(value))) { ElMessage.warning('请输入有效的数值'); return; }
  const identifier = point.value.identifier, displayName = point.value.name || identifier, generation = epoch, id = props.deviceId;
  sending.value = true;
  try {
    await ElMessageBox.confirm(`设备：${props.deviceName}\n点位：${displayName}（${identifier}）\n下发值：${String(value)}${point.value.unit || ''}\n操作将写入实际设备，请确认。`, '确认下发', { confirmButtonText: '确认下发', cancelButtonText: '取消', type: 'warning' });
    if (generation !== epoch || id !== props.deviceId || !visible.value) return;
    requestId.value = newRequestId(); submitted.value = true;
    const result = await apiRequest<Task>({ url: `/api/devices/${id}/commands`, method: 'POST', data: { requestId: requestId.value, metric: identifier, value }, silentError: true });
    if (generation !== epoch || id !== props.deviceId) return;
    task.value = result; notify(result);
    if (result.status === 'SENT') ElMessage.info('指令已提交，正在等待网关执行回执');
    await refresh(true);
  } catch (error) {
    if (submitted.value) {
      const response = (error as { response?: { status?: number; data?: { message?: string } } })?.response;
      if (response?.status && response.status >= 400 && response.status < 500) ElMessage.error(response.data?.message || '指令未通过校验，未执行');
      else ElMessage.warning('提交结果未确认，请查询下发记录，不要重复下发');
      await refresh(true);
    }
  } finally { if (generation === epoch) sending.value = false; }
}
async function toggle(metric: string, value: boolean) {
  const target = points.value.find(item => item.identifier === metric);
  if (!available.value || target?.dataType !== 'BOOLEAN' || busyBoolean.has(metric)) return;
  const id = props.deviceId, generation = epoch, displayName = target.name || metric;
  setBooleanBusy(metric, true);
  let submittedBoolean = false;
  try {
    await ElMessageBox.confirm(`设备：${props.deviceName}\n点位：${displayName}（${metric}）\n切换为：${value ? '开启（true）' : '关闭（false）'}\n操作将写入实际设备，请确认。`, '确认切换', { confirmButtonText: '确认下发', cancelButtonText: '取消', type: 'warning' });
    if (disposed || generation !== epoch || id !== props.deviceId || !available.value || !points.value.some(item => item.identifier === metric && item.dataType === 'BOOLEAN')) return;
    const commandId = newRequestId();
    booleanRequests.set(metric, { requestId: commandId, startedAt: Date.now() });
    submittedBoolean = true;
    const result = await apiRequest<Task>({ url: `/api/devices/${id}/commands`, method: 'POST', data: { requestId: commandId, metric, value }, silentError: true });
    if (disposed || id !== props.deviceId) return;
    if (result.status === 'SENT') ElMessage.info('指令已提交，正在等待网关执行回执');
    else reconcileBooleanRequests([result]);
    await refresh(true);
  } catch (error) {
    if (submittedBoolean && !disposed && id === props.deviceId) {
      const response = (error as { response?: { status?: number; data?: { message?: string } } })?.response;
      if (response?.status && response.status >= 400 && response.status < 500) {
        booleanRequests.delete(metric); setBooleanBusy(metric, false);
        ElMessage.error(response.data?.message || '指令未通过校验，未执行');
      } else {
        ElMessage.warning('提交结果未确认，请查询下发记录，不要重复下发');
        await refresh(true);
      }
    }
  } finally {
    if (!submittedBoolean && id === props.deviceId) setBooleanBusy(metric, false);
    if (!disposed && id === props.deviceId) poll();
  }
}
watch(visible, value => { if (!value) { stop(); epoch++; sending.value = false; poll(); } });
watch(historyVisible, value => { stop(); if (value) void refresh(); poll(); });
watch(() => props.deviceId, async () => { visible.value = false; historyVisible.value = false; epoch++; stop(); task.value = undefined; points.value = []; available.value = false; booleanRequests.clear(); busyBoolean.clear(); emit('pending', []); emit('capabilities', { points: [], available: false, reason: '' }); const generation = epoch; await refresh(true); if (generation === epoch) poll(); }, { immediate: true });
onBeforeUnmount(() => { disposed = true; epoch++; stop(); });
defineExpose({ open, refresh, toggle, showHistory });
</script>

<template>
  <el-dialog v-model="visible" title="点位下发" width="min(500px, 94vw)" :close-on-click-modal="false" :close-on-press-escape="!sending" :show-close="!sending" append-to-body>
    <div v-loading="loading" class="command-content">
      <p class="command-description">{{ deviceName }} · 写入实际设备，以网关回执为准</p>
      <el-alert v-if="reason" :title="reason" type="warning" :closable="false" />
      <el-form label-position="top" class="command-form">
        <el-form-item label="下发点位"><div class="command-point"><strong>{{ point?.name || selected }}</strong><small v-if="point?.name && point.name !== selected">{{ selected }}</small></div></el-form-item>
        <el-form-item :label="`下发值${point?.unit ? `（${point.unit}）` : ''}`">
          <el-switch v-if="point?.dataType === 'BOOLEAN'" v-model="boolValue" :disabled="sending || submitted" active-text="true" inactive-text="false" aria-label="布尔下发值" @change="boolTouched = true" />
          <el-input v-else v-model="input" :disabled="sending || submitted" :maxlength="2048" :placeholder="point?.dataType === 'NUMBER' ? '输入数值（不会自动四舍五入）' : '输入下发值'" />
        </el-form-item>
      </el-form>
      <el-alert v-if="task" :title="names[task.status]" :description="task.message" :type="task.status === 'SUCCEEDED' ? 'success' : task.status === 'FAILED' ? 'error' : 'warning'" :closable="false" show-icon />
      <div class="command-actions"><el-button @click="refresh()">查询下发结果</el-button><el-button type="primary" :loading="sending" :disabled="!available || !point || submitted || (point.dataType === 'BOOLEAN' && !boolTouched)" @click="send">{{ submitted ? '已提交，请查看回执' : '确认下发' }}</el-button></div>
      <div class="recent-heading"><strong>最近记录</strong><el-button link type="primary" @click="showHistory">查看历史 →</el-button></div>
      <div v-if="recentTasks.length" class="recent-list">
        <div v-for="item in recentTasks" :key="item.id" class="recent-row" :title="item.message || names[item.status]">
          <time>{{ new Date(item.createdAt).toLocaleString('zh-CN', { hour12: false }) }}</time><span>{{ String(item.value) }}</span><em :class="item.status.toLowerCase()">{{ names[item.status] || item.status }}</em>
        </div>
      </div>
      <p v-else class="recent-empty">暂无下发记录</p>
    </div>
  </el-dialog>
  <el-dialog v-model="historyVisible" title="下发记录" width="min(860px, 94vw)" append-to-body>
    <div class="command-history-heading"><p>查看该设备最近 30 条点位下发及网关执行回执</p><el-button @click="refresh()">刷新记录</el-button></div>
    <el-table v-loading="historyLoading" :data="allTasks" empty-text="暂无下发记录" class="command-history-table">
      <el-table-column label="时间" min-width="170"><template #default="{ row }">{{ new Date(row.createdAt).toLocaleString('zh-CN', { hour12: false }) }}</template></el-table-column>
      <el-table-column prop="metric" label="点位" min-width="125" show-overflow-tooltip />
      <el-table-column label="下发值" min-width="110"><template #default="{ row }">{{ String(row.value) }}</template></el-table-column>
      <el-table-column label="结果" min-width="130"><template #default="{ row }"><el-tag :type="row.status === 'SUCCEEDED' ? 'success' : row.status === 'FAILED' ? 'danger' : 'warning'" effect="plain">{{ names[row.status] || row.status }}</el-tag></template></el-table-column>
      <el-table-column prop="message" label="网关回执" min-width="250" show-overflow-tooltip />
    </el-table>
  </el-dialog>
</template>

<style scoped>
.command-description { margin: 0 0 14px; color: #64748b; font-size: 13px; line-height: 1.5; }
.command-form { margin-top: 18px; }
.command-form :deep(.el-form-item) { margin-bottom: 17px; }
.command-point { width: 100%; padding: 10px 12px; border-radius: 8px; background: #f6f8fb; color: #334155; }
.command-point strong { font-size: 14px; }
.command-point small { display: block; margin-top: 3px; color: #94a3b8; }
.command-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 20px; }
.recent-heading { display: flex; align-items: center; justify-content: space-between; margin-top: 24px; padding-top: 13px; border-top: 1px solid #edf1f5; }
.recent-heading strong { font-size: 13px; color: #334155; }
.recent-list { margin-top: 2px; }
.recent-row { display: grid; grid-template-columns: 1fr auto 84px; align-items: center; gap: 12px; min-height: 34px; border-bottom: 1px solid #f2f4f7; color: #64748b; font-size: 12px; }
.recent-row span { max-width: 110px; overflow: hidden; color: #334155; text-overflow: ellipsis; white-space: nowrap; }
.recent-row em { font-style: normal; text-align: right; }
.recent-row em.succeeded { color: #16a34a; }.recent-row em.failed { color: #e34d59; }.recent-row em.timeout { color: #d97706; }
.recent-empty { margin: 7px 0 0; color: #94a3b8; font-size: 12px; }
.command-history-heading { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 18px; }
.command-history-heading p { margin: 0; color: #94a3b8; font-size: 13px; }
@media (max-width: 600px) { .command-history-heading { align-items: flex-start; } .command-history-table { width: 100%; overflow-x: auto; } }
</style>
