<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { Edit, Plus, RefreshRight } from '@element-plus/icons-vue';
import { ElMessage } from 'element-plus';
import { apiRequest } from '../api/request';

interface Plan { id: string; code: string; name: string; description?: string; deviceLimit: number; pointLimit: number; scadaProjectLimit: number; userLimit: number; monthlyPriceCents: number; priceCents: number; billingCycles:Array<'MONTHLY'|'YEARLY'>; quoteOnly:boolean; currency: string; enabled: boolean; sortOrder: number; gracePeriodDays: number; historyRetentionDays:number; historyRetentionMonths:number; _count?: { tenants: number; subscriptions: number; orders: number } }
const plans = ref<Plan[]>([]);
const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const editingId = ref('');
const planStatus = ref('ALL');
const form = reactive({ code: '', name: '', description: '', deviceLimit: 0, pointLimit: 0, scadaProjectLimit: 0, userLimit:0, monthlyPrice:0, price: 0, billingCycles:['YEARLY'] as Array<'MONTHLY'|'YEARLY'>, quoteOnly:false, enabled: true, sortOrder: 10, gracePeriodDays:7, historyRetentionDays:7, historyRetentionMonths:0 });

const filteredPlans = computed(() => plans.value.filter(plan => !['ENABLED', 'DISABLED'].includes(planStatus.value) || plan.enabled === (planStatus.value === 'ENABLED')));
async function load() { loading.value = true; try { plans.value = await apiRequest({ url: '/api/subscriptions/plans', method: 'GET' }); } finally { loading.value = false; } }
function openCreate() { editingId.value = ''; Object.assign(form, { code: '', name: '', description: '', deviceLimit: 0, pointLimit: 0, scadaProjectLimit: 0, userLimit:0, monthlyPrice:0, price: 0, billingCycles:['YEARLY'], quoteOnly:false, enabled: true, sortOrder: (plans.value.length + 1) * 10, gracePeriodDays:7, historyRetentionDays:7, historyRetentionMonths:0 }); dialogVisible.value = true; }
function openEdit(plan: Plan) { editingId.value = plan.id; Object.assign(form, { code: plan.code, name: plan.name, description: plan.description || '', deviceLimit: plan.deviceLimit, pointLimit: plan.pointLimit, scadaProjectLimit: plan.scadaProjectLimit, userLimit:plan.userLimit, monthlyPrice:plan.monthlyPriceCents/100, price: plan.priceCents / 100, billingCycles:[...plan.billingCycles], quoteOnly:plan.quoteOnly, enabled: plan.enabled, sortOrder: plan.sortOrder, gracePeriodDays:plan.gracePeriodDays, historyRetentionDays:plan.historyRetentionDays, historyRetentionMonths:plan.historyRetentionMonths }); dialogVisible.value = true; }
async function save() {
  if (!form.code.trim() || !form.name.trim()) return ElMessage.warning('请填写版本编码和名称');
  if (!form.billingCycles.length) return ElMessage.warning('请至少选择一种可售周期');
  saving.value = true;
  try {
    const data = { ...form, monthlyPriceCents:Math.round(Number(form.monthlyPrice)*100), priceCents: Math.round(Number(form.price) * 100) } as Record<string, unknown>;
    delete data.price; delete data.monthlyPrice;
    await apiRequest({ url: editingId.value ? `/api/subscriptions/plans/${editingId.value}` : '/api/subscriptions/plans', method: editingId.value ? 'PATCH' : 'POST', data });
    ElMessage.success(editingId.value ? '版本已更新' : '版本已创建'); dialogVisible.value = false; await load();
  } finally { saving.value = false; }
}
const quota = (value: number, unit: string) => value === 0 ? '不限' : `${value.toLocaleString()} ${unit}`;
const retention=(p:Plan)=>p.historyRetentionMonths?`${p.historyRetentionMonths}个自然月`:`${p.historyRetentionDays||7}天`;
const money = (cents: number) => `¥${(cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })}`;
onMounted(load);
</script>

<template>
  <div>
    <div class="page-actions">
      <div><h1 class="page-title">套餐管理</h1><p class="page-description">设置套餐月付/年付价格、资源额度与到期宽限期；0 表示该项不限制</p></div>
      <div class="header-actions">
        <el-select v-model="planStatus" aria-label="套餐状态" title="按套餐状态筛选">
          <el-option label="全部状态" value="ALL"/>
          <el-option label="可用" value="ENABLED"/>
          <el-option label="停用" value="DISABLED"/>
        </el-select>
        <el-button :icon="RefreshRight" @click="load">刷新</el-button>
        <el-button type="primary" :icon="Plus" @click="openCreate">新增套餐</el-button>
      </div>
    </div>
    <section class="plan-panel" v-loading="loading">
      <div v-if="filteredPlans.length" class="plan-grid">
        <article v-for="plan in filteredPlans" :key="plan.id" class="plan-card" :class="{ disabled: !plan.enabled }">
          <div class="plan-head"><div><span class="plan-code">{{ plan.code }}</span><h2>{{ plan.name }}</h2></div><el-tag :type="plan.enabled ? 'success' : 'info'">{{ plan.enabled ? '可用' : '停用' }}</el-tag></div>
          <p class="description">{{ plan.description || '暂无版本说明' }}</p>
          <strong class="price">{{ plan.quoteOnly ? '商务报价' : plan.billingCycles.includes('YEARLY') ? money(plan.priceCents) : money(plan.monthlyPriceCents) }}</strong><span class="price-note">{{ plan.quoteOnly ? '联系商务后开通' : plan.billingCycles.map(v=>v==='YEARLY'?'年付':'月付').join(' · ') }}</span>
          <div class="quota-grid"><div><span>历史保存</span><b>{{retention(plan)}}</b></div><div><span>设备数量</span><b>{{ quota(plan.deviceLimit, '台') }}</b></div><div><span>点位数量</span><b>{{ quota(plan.pointLimit, '个') }}</b></div><div><span>组态项目</span><b>{{ quota(plan.scadaProjectLimit, '个') }}</b></div><div><span>用户数量</span><b>{{ quota(plan.userLimit, '人') }}</b></div></div>
          <div class="plan-foot"><span>{{ plan._count?.subscriptions || 0 }} 个有效订阅 · {{ plan.gracePeriodDays }} 天宽限期</span><el-button link type="primary" :icon="Edit" @click="openEdit(plan)">编辑</el-button></div>
        </article>
      </div>
      <el-empty v-else :description="['ENABLED', 'DISABLED'].includes(planStatus) ? '当前状态下暂无套餐' : '暂无套餐，请先新增'" />
    </section>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑套餐' : '新增套餐'" width="620px" destroy-on-close>
      <el-form label-position="top">
        <div class="form-grid"><el-form-item label="版本编码" required><el-input v-model="form.code" maxlength="40" placeholder="例如 PROFESSIONAL" /></el-form-item><el-form-item label="版本名称" required><el-input v-model="form.name" maxlength="60" placeholder="例如 专业版" /></el-form-item></div>
        <el-form-item label="版本说明"><el-input v-model="form.description" type="textarea" maxlength="300" show-word-limit :rows="2" /></el-form-item>
        <div class="form-grid"><el-form-item label="设备上限（0 不限）"><el-input-number v-model="form.deviceLimit" :min="0" :max="1000000" controls-position="right" /></el-form-item><el-form-item label="点位上限（0 不限）"><el-input-number v-model="form.pointLimit" :min="0" :max="100000000" controls-position="right" /></el-form-item><el-form-item label="组态项目上限（0 不限）"><el-input-number v-model="form.scadaProjectLimit" :min="0" :max="1000000" controls-position="right" /></el-form-item><el-form-item label="用户上限（0 不限）"><el-input-number v-model="form.userLimit" :min="0" :max="1000000" controls-position="right" /></el-form-item></div>
        <div class="form-grid"><el-form-item label="历史保存天数（按月时填0）"><el-input-number v-model="form.historyRetentionDays" :min="0" :max="3660"/></el-form-item><el-form-item label="历史保存自然月数（按天时填0）"><el-input-number v-model="form.historyRetentionMonths" :min="0" :max="120"/></el-form-item></div>
        <p class="description">天数和自然月数仅设置一项；缩短保存期限后给予7天清理宽限期。升级不能恢复已清理的数据。</p>
        <el-form-item label="可售周期"><el-checkbox-group v-model="form.billingCycles"><el-checkbox value="YEARLY">年付</el-checkbox><el-checkbox value="MONTHLY">月付</el-checkbox></el-checkbox-group></el-form-item>
        <div class="form-grid"><el-form-item label="月付价格（元）"><el-input-number v-model="form.monthlyPrice" :disabled="form.quoteOnly||!form.billingCycles.includes('MONTHLY')" :min="0" :max="10000000" :precision="2" controls-position="right" /></el-form-item><el-form-item label="年付价格（元）"><el-input-number v-model="form.price" :disabled="form.quoteOnly||!form.billingCycles.includes('YEARLY')" :min="0" :max="10000000" :precision="2" controls-position="right" /></el-form-item><el-form-item label="到期宽限期（天）"><el-input-number v-model="form.gracePeriodDays" :min="0" :max="90" controls-position="right" /></el-form-item><el-form-item label="排序"><el-input-number v-model="form.sortOrder" :min="0" :max="10000" controls-position="right" /></el-form-item></div>
        <el-form-item label="销售方式"><el-switch v-model="form.quoteOnly" active-text="商务报价" inactive-text="在线下单" /></el-form-item>
        <el-form-item label="状态"><el-switch v-model="form.enabled" active-text="可新购/升级" inactive-text="停止销售" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存</el-button></template>
    </el-dialog>

  </div>
</template>

<style scoped>
.plan-panel { min-height: 260px; }.plan-grid { display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:18px }.plan-card{padding:22px;border:1px solid #e1e9f4;border-radius:14px;background:#fff;box-shadow:0 6px 22px rgba(38,76,125,.05)}.plan-card.disabled{opacity:.68}.plan-head,.plan-foot{display:flex;align-items:center;justify-content:space-between;gap:12px}.plan-head h2{margin:5px 0 0;font-size:20px;color:#172b4d}.plan-code{font-size:11px;color:#3b82f6;letter-spacing:.08em}.description{min-height:42px;color:#75859c;font-size:13px;line-height:1.65}.price{font-size:28px;color:#1769e0}.price-note{margin-left:8px;color:#96a2b4;font-size:12px}.quota-grid{display:grid;gap:8px;margin:18px 0;padding:14px;border-radius:10px;background:#f6f9fd}.quota-grid div{display:flex;justify-content:space-between;color:#65758b;font-size:13px}.quota-grid b{color:#203754}.plan-foot{padding-top:14px;border-top:1px solid #edf1f6;color:#8592a6;font-size:12px}.form-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}.form-grid.three{grid-template-columns:repeat(3,minmax(0,1fr))}.form-grid :deep(.el-input-number){width:100%}@media(max-width:1100px){.plan-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:700px){.plan-grid,.form-grid,.form-grid.three{grid-template-columns:1fr}}
.header-actions{display:flex;align-items:center;gap:10px}.header-actions .el-select{width:150px}.header-actions .el-button+.el-button{margin-left:0}@media(max-width:700px){.page-actions{align-items:flex-start;flex-direction:column;gap:16px}.header-actions{width:100%;flex-wrap:wrap}.header-actions .el-select{flex:1;min-width:140px}}
</style>
