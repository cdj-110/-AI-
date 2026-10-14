<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { QuestionFilled } from '@element-plus/icons-vue';
import { apiRequest } from '../api/request';
const props = defineProps<{ rule?: Record<string, any> }>();
const emit = defineEmits<{ saved: []; cancel: [] }>();
interface Device { id: string; name: string; deviceKey: string; deviceType: string }
interface Point { identifier: string; name: string; dataType: string }
const form = reactive({ name: '', sourceDeviceId: '', sourceMetric: '', operator: 'GT', threshold: 40, targetDeviceId: '', targetMetric: '', actionValue: false as boolean | number | string });
const sources = ref<Device[]>([]), targets = ref<Device[]>([]), sourcePoints = ref<Point[]>([]), targetPoints = ref<Point[]>([]);
const saving = ref(false), loadingSource = ref(false), loadingTarget = ref(false), error = ref(''), targetHint = ref('');
const targetType = computed(() => targetPoints.value.find(p => p.identifier === form.targetMetric)?.dataType);
const typeLabels: Record<string, string> = { BOOLEAN: '布尔值', NUMBER: '数值', STRING: '文本' };
const operatorLabels: Record<string, string> = { GT: '大于', GTE: '大于等于', LT: '小于', LTE: '小于等于', EQ: '等于', NE: '不等于' };
const summary = computed(() => {
  if (!form.sourceMetric || !form.targetMetric) return '';
  const source = sources.value.find(d => d.id === form.sourceDeviceId)?.name || form.sourceDeviceId;
  const target = targets.value.find(d => d.id === form.targetDeviceId)?.name || form.targetDeviceId;
  return `${source} / ${form.sourceMetric} ${operatorLabels[form.operator]} ${form.threshold} 时，向 ${target} / ${form.targetMetric} 下发 ${JSON.stringify(form.actionValue)}`;
});
let sourceGeneration = 0, targetGeneration = 0, sourceSearch = 0, targetSearch = 0;
async function search(kind: 'source' | 'target', keyword = '') {
  const generation = kind === 'source' ? ++sourceSearch : ++targetSearch;
  const data = await apiRequest<{ items: Device[] }>({ url: '/api/devices', method: 'GET', params: { page: 1, pageSize: 100, keyword, ...(kind === 'target' ? { deviceType: 'GATEWAY_CHILD' } : {}) } });
  if (generation !== (kind === 'source' ? sourceSearch : targetSearch)) return;
  const list = kind === 'source' ? sources : targets, selected = kind === 'source' ? form.sourceDeviceId : form.targetDeviceId;
  list.value = [...new Map([...list.value.filter(d => d.id === selected), ...data.items.filter(d => d.deviceType !== 'GATEWAY')].map(d => [d.id, d])).values()];
}
async function sourceChanged(clear = true) {
  const generation = ++sourceGeneration; sourcePoints.value = []; if (clear) form.sourceMetric = '';
  if (!form.sourceDeviceId) return;
  loadingSource.value = true;
  try { const points = await apiRequest<Array<Point & { enabled: boolean; ignored: boolean }>>({ url: `/api/devices/${form.sourceDeviceId}/metrics`, method: 'GET' }); if (generation === sourceGeneration) sourcePoints.value = points.filter(p => p.enabled && !p.ignored && p.dataType === 'NUMBER'); }
  catch (e: any) { if (generation === sourceGeneration) error.value = e.response?.data?.message || '来源点位加载失败'; }
  finally { if (generation === sourceGeneration) loadingSource.value = false; }
}
async function targetChanged(clear = true) {
  const generation = ++targetGeneration; targetPoints.value = []; targetHint.value = ''; if (clear) form.targetMetric = '';
  if (!form.targetDeviceId) return;
  loadingTarget.value = true;
  try { const caps = await apiRequest<{ points: Point[]; reason: string }>({ url: `/api/devices/${form.targetDeviceId}/commands/capabilities`, method: 'GET' }); if (generation === targetGeneration) { targetPoints.value = caps.points; targetHint.value = caps.reason; } }
  catch (e: any) { if (generation === targetGeneration) error.value = e.response?.data?.message || '可写点位加载失败'; }
  finally { if (generation === targetGeneration) loadingTarget.value = false; }
}
function pointChanged() { form.actionValue = targetType.value === 'BOOLEAN' ? false : targetType.value === 'NUMBER' ? 0 : ''; }
async function save() {
  error.value = '';
  if (!form.name.trim() || !form.sourceMetric || !form.targetMetric || !Number.isFinite(form.threshold)) { error.value = '请完整填写规则名称、点位、条件和指令值'; return; }
  if (targetType.value === 'NUMBER' && (typeof form.actionValue !== 'number' || !Number.isFinite(form.actionValue))) { error.value = '请输入有效数字指令值'; return; }
  saving.value = true;
  try { await apiRequest({ url: props.rule ? `/api/device-linkage/${props.rule.id}` : '/api/device-linkage', method: props.rule ? 'PATCH' : 'POST', data: { ...form, name: form.name.trim() } }); ElMessage.success('规则已保存，当前为停用状态'); emit('saved'); }
  catch (e: any) { error.value = e.response?.data?.message || '保存失败'; }
  finally { saving.value = false; }
}
onMounted(async () => {
  if (props.rule) for (const key of Object.keys(form)) if (props.rule[key] !== undefined) (form as any)[key] = props.rule[key];
  try {
    await Promise.all([search('source'), search('target')]);
    for (const kind of ['source', 'target'] as const) { const id = kind === 'source' ? form.sourceDeviceId : form.targetDeviceId, list = kind === 'source' ? sources : targets; if (id && !list.value.some(d => d.id === id)) list.value.push(await apiRequest<Device>({ url: `/api/devices/${id}`, method: 'GET' })); }
    await Promise.all([sourceChanged(false), targetChanged(false)]);
  } catch (e: any) { error.value = e.response?.data?.message || '设备加载失败'; }
});
</script>
<template>
  <el-form label-position="top" class="linkage-form" @submit.prevent="save">
    <div class="editor-scroll">
      <div class="editor-meta">
        <div><el-tag effect="plain">云端执行</el-tag><el-tag type="info" effect="plain">默认停用</el-tag></div>
        <el-popover trigger="click" width="340" placement="bottom-end">
          <template #reference><el-button link :icon="QuestionFilled" aria-label="查看联动规则说明">规则说明</el-button></template>
          <div class="rule-help"><strong>运行与安全说明</strong><ul><li>启用后独立于组态页面运行，依赖设备云连接；不替代安全联锁。</li><li>仅使用启用后30秒内的有效数值上报，不补发历史数据。</li><li>首次满足或从不满足变为满足时下发一次，持续满足不重复。同一目标点位至少间隔30秒，期间触发不补发。</li><li>禁止同点位自联动或闭环。失败或结果未知不自动重试，设备状态以实际上报为准。</li></ul></div>
        </el-popover>
      </div>
      <section class="config-section">
        <div class="section-heading"><span class="step-number">1</span><h3>基本信息</h3></div>
        <el-form-item label="规则名称" required><el-input v-model="form.name" maxlength="80" show-word-limit placeholder="例如：噪声超限关闭设备" /></el-form-item>
      </section>
      <section class="config-section">
        <div class="section-heading"><span class="step-number">2</span><h3>触发条件</h3><span class="section-note">当来源点位满足条件时</span></div>
        <div class="point-row"><el-form-item label="来源设备" required><el-select v-model="form.sourceDeviceId" filterable remote :remote-method="(v:string)=>search('source',v)" placeholder="搜索设备名称" @change="sourceChanged()"><el-option v-for="d in sources" :key="d.id" :value="d.id" :label="`${d.name} · ${d.deviceKey}`" /></el-select></el-form-item><el-form-item label="数值点位" required><el-select v-model="form.sourceMetric" :loading="loadingSource" :disabled="!form.sourceDeviceId" filterable placeholder="选择已启用的数值点位"><el-option v-for="p in sourcePoints" :key="p.identifier" :value="p.identifier" :label="`${p.name} / ${p.identifier}`" /></el-select></el-form-item></div>
        <div class="point-row last-row"><el-form-item label="判断条件"><el-select v-model="form.operator"><el-option v-for="(label,value) in operatorLabels" :key="value" :value="value" :label="label" /></el-select></el-form-item><el-form-item label="阈值" required><el-input-number v-model="form.threshold" :controls="false" /></el-form-item></div>
      </section>
      <section class="config-section">
        <div class="section-heading"><span class="step-number">3</span><h3>执行动作</h3><span class="section-note">下发点位值</span></div>
        <div class="point-row"><el-form-item label="目标采集子设备" required><el-select v-model="form.targetDeviceId" filterable remote :remote-method="(v:string)=>search('target',v)" placeholder="搜索目标设备" @change="targetChanged()"><el-option v-for="d in targets" :key="d.id" :value="d.id" :label="`${d.name} · ${d.deviceKey}`" /></el-select></el-form-item><el-form-item label="可写点位" required><el-select v-model="form.targetMetric" :loading="loadingTarget" :disabled="!form.targetDeviceId" filterable placeholder="以网关实际可写配置为准" @change="pointChanged"><el-option v-for="p in targetPoints" :key="p.identifier" :value="p.identifier" :label="`${p.name} / ${p.identifier}`" /></el-select></el-form-item></div>
        <el-alert v-if="targetHint" :title="targetHint" type="warning" :closable="false" class="target-hint" />
        <el-form-item required class="value-field"><template #label>下发值 <el-tag v-if="targetType" size="small" type="info">{{typeLabels[targetType]}}</el-tag></template><el-select v-if="targetType==='BOOLEAN'" v-model="form.actionValue"><el-option label="true" :value="true"/><el-option label="false" :value="false"/></el-select><el-input-number v-else-if="targetType==='NUMBER'" v-model="form.actionValue as number" :controls="false"/><el-input v-else-if="targetType==='STRING'" v-model="form.actionValue as string" maxlength="2048"/><el-input v-else disabled placeholder="选择目标点位后设置下发值" /></el-form-item>
      </section>
      <div v-if="summary" class="rule-preview"><span>规则预览</span><p>{{summary}}</p></div>
    </div>
    <div class="editor-footer">
      <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
      <div class="actions"><span>保存后需另行授权启用</span><div><el-button :disabled="saving" @click="emit('cancel')">取消</el-button><el-button type="primary" native-type="submit" :loading="saving" :disabled="loadingSource||loadingTarget">保存规则</el-button></div></div>
    </div>
  </el-form>
</template>
<style scoped>
.linkage-form{display:flex;flex-direction:column;max-height:min(740px,calc(90dvh - 100px));overflow:hidden}
.editor-scroll{min-height:0;overflow:auto;padding:0 10px 4px 0;scrollbar-width:thin;scrollbar-color:#ccd5e1 transparent}
.editor-meta,.editor-meta>div{display:flex;align-items:center;gap:8px}.editor-meta{justify-content:space-between;margin:0 0 16px}
.config-section{border:1px solid #e5eaf2;border-radius:10px;padding:16px 18px;margin-bottom:12px;background:#fff}
.section-heading{display:flex;align-items:center;gap:8px;margin-bottom:14px}.step-number{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:22px;height:22px;border-radius:6px;background:#edf5ff;color:#337ecc;font-size:12px;font-weight:600}
h3{font-size:14px;color:#26364d;margin:0;font-weight:600}.section-note{font-size:12px;color:#8794a7;margin-left:auto}
.point-row{display:grid;grid-template-columns:1fr 1fr;gap:16px}.el-select,.el-input-number{width:100%}
:deep(.el-form-item){margin-bottom:14px}:deep(.el-form-item__label){padding-bottom:6px;font-size:13px;color:#536177}
.config-section>.el-form-item:last-child,.last-row .el-form-item{margin-bottom:0}.value-field{max-width:calc(50% - 8px)}.value-field :deep(.el-form-item__label){display:flex;align-items:center;gap:8px}
:deep(.el-input-number .el-input__inner){text-align:left}.target-hint{margin-bottom:14px}
.rule-preview{background:#f4f8ff;border:1px solid #e0ebfc;border-radius:8px;padding:12px 14px;margin-bottom:12px;color:#445977;font-size:12px}.rule-preview>span{font-weight:600;color:#337ecc}.rule-preview p{margin:6px 0 0;line-height:1.7;overflow-wrap:anywhere}
.editor-footer{flex-shrink:0;background:#fff;border-top:1px solid #e8edf4;padding:14px 0 0;margin-top:8px}.editor-footer .el-alert{margin-bottom:10px}
.actions{display:flex;justify-content:space-between;align-items:center;gap:12px}.actions>span{font-size:12px;color:#8794a7}.actions>div{display:flex;flex-shrink:0;gap:8px}.actions .el-button+.el-button{margin-left:0}
.rule-help{font-size:12px;color:#536177;line-height:1.8}.rule-help strong{color:#26364d}.rule-help ul{padding-left:18px;margin:8px 0 0}.rule-help li+li{margin-top:6px}
@media(max-width:650px){.point-row{grid-template-columns:1fr;gap:0}.config-section{padding:14px}.section-note{display:none}.value-field{max-width:none}.last-row .el-form-item:not(:last-child){margin-bottom:14px}.actions{flex-wrap:wrap}.actions>div{margin-left:auto}.editor-scroll{padding-right:6px}}
</style>
