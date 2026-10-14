<script setup lang="ts">
import { Check, Delete, Edit, Plus, RefreshRight, Search } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { computed, onMounted, reactive, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { apiRequest } from '../api/request';
import { isAdminPortal } from '../portal';

type Surface = 'ADMIN' | 'TENANT' | 'GATEWAY';
type Category = 'FEATURE' | 'IMPROVEMENT' | 'FIX' | 'SECURITY';
type Status = 'DRAFT' | 'PUBLISHED';

interface ChangeItem { text: string }
interface ChangelogEntry {
  id: string;
  version?: string;
  title: string;
  summary?: string;
  releaseDate: string;
  surfaces: Surface[];
  category: Category;
  items: ChangeItem[];
  status: Status;
  createdByName: string;
  publishedAt?: string;
  createdAt: string;
}

const surfaceOptions: Array<{ value: Surface; label: string; short: string }> = [
  { value: 'ADMIN', label: '管理平台', short: '管理端' },
  { value: 'TENANT', label: '用户端', short: '用户端' },
  { value: 'GATEWAY', label: '网关', short: '网关端' },
];
const route = useRoute();
const canManage = computed(() => isAdminPortal && route.meta.manageChangelog === true);
const categoryMap: Record<Category, { label: string; tone: string }> = {
  FEATURE: { label: '新功能', tone: 'feature' },
  IMPROVEMENT: { label: '优化', tone: 'improvement' },
  FIX: { label: '修复', tone: 'fix' },
  SECURITY: { label: '安全', tone: 'security' },
};

const loading = ref(false);
const saving = ref(false);
const entries = ref<ChangelogEntry[]>([]);
const surface = ref('');
const category = ref('');
const status = ref('');
const keyword = ref('');
const dialogVisible = ref(false);
const editingId = ref('');
const form = reactive({
  version: '', title: '', summary: '', releaseDate: '', surfaces: [] as Surface[],
  category: 'FEATURE' as Category, status: 'PUBLISHED' as Status, items: [{ text: '' }] as ChangeItem[],
});

const groupedEntries = computed(() => entries.value.reduce<Array<{ date: string; entries: ChangelogEntry[] }>>((groups, entry) => {
  const date = dateKey(entry.releaseDate);
  const previous = groups[groups.length - 1];
  if (previous?.date === date) previous.entries.push(entry);
  else groups.push({ date, entries: [entry] });
  return groups;
}, []));

async function load() {
  loading.value = true;
  try {
    const result = await apiRequest<{ items: ChangelogEntry[]; total: number }>({
      url: canManage.value ? '/api/changelog/manage' : '/api/changelog', method: 'GET', params: {
        surface: surface.value || undefined,
        category: category.value || undefined,
        status: canManage.value ? status.value || undefined : undefined,
        keyword: keyword.value.trim() || undefined,
      }, silentError: !canManage.value,
    });
    entries.value = result.items.map(item => ({ ...item, items: normalizeItems(item.items) }));
  } finally { loading.value = false; }
}

function normalizeItems(value: unknown): ChangeItem[] {
  if (!Array.isArray(value)) return [];
  return value.map(item => typeof item === 'string' ? { text: item } : { text: String((item as ChangeItem)?.text || '') }).filter(item => item.text);
}

function openCreate() {
  editingId.value = '';
  Object.assign(form, {
    version: '', title: '', summary: '', releaseDate: localDate(new Date()), surfaces: ['TENANT'] as Surface[],
    category: 'FEATURE' as Category, status: 'PUBLISHED' as Status, items: [{ text: '' }],
  });
  dialogVisible.value = true;
}

function openEdit(entry: ChangelogEntry) {
  editingId.value = entry.id;
  Object.assign(form, {
    version: entry.version || '', title: entry.title, summary: entry.summary || '', releaseDate: localDate(new Date(entry.releaseDate)),
    surfaces: [...entry.surfaces], category: entry.category, status: entry.status,
    items: entry.items.map(item => ({ text: item.text })),
  });
  dialogVisible.value = true;
}

function addItem() { if (form.items.length < 30) form.items.push({ text: '' }); }
function removeItem(index: number) { if (form.items.length > 1) form.items.splice(index, 1); else form.items[0].text = ''; }

async function save() {
  const items = form.items.map(item => ({ text: item.text.trim() })).filter(item => item.text);
  if (!form.title.trim()) return ElMessage.warning('请输入更新标题');
  if (!form.releaseDate) return ElMessage.warning('请选择发布日期');
  if (!form.surfaces.length) return ElMessage.warning('至少选择一个适用端');
  if (!items.length) return ElMessage.warning('至少填写一条更新内容');
  saving.value = true;
  try {
    const payload = {
      version: form.version.trim() || undefined,
      title: form.title.trim(), summary: form.summary.trim() || undefined,
      releaseDate: `${form.releaseDate}T00:00:00+08:00`, surfaces: form.surfaces,
      category: form.category, status: form.status, items,
    };
    await apiRequest({ url: editingId.value ? `/api/changelog/${editingId.value}` : '/api/changelog', method: editingId.value ? 'PATCH' : 'POST', data: payload });
    ElMessage.success(editingId.value ? '更新日志已保存' : (form.status === 'PUBLISHED' ? '更新日志已发布' : '草稿已保存'));
    dialogVisible.value = false;
    await load();
  } finally { saving.value = false; }
}

async function remove(entry: ChangelogEntry) {
  try {
    await ElMessageBox.confirm(`确认删除“${entry.title}”吗？删除后无法恢复。`, '删除更新日志', { type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消' });
  } catch { return; }
  await apiRequest({ url: `/api/changelog/${entry.id}`, method: 'DELETE' });
  ElMessage.success('更新日志已删除');
  await load();
}

function localDate(value: Date) {
  const offset = value.getTimezoneOffset() * 60_000;
  return new Date(value.getTime() - offset).toISOString().slice(0, 10);
}
function dateKey(value: string) { return localDate(new Date(value)); }
function dateLabel(value: string) {
  const date = new Date(`${value}T00:00:00+08:00`);
  return new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'long', day: 'numeric', weekday: 'short' }).format(date);
}
function surfaceLabel(value: Surface) { return surfaceOptions.find(item => item.value === value)?.short || value; }

watch([surface, category, status], load);
onMounted(load);
</script>

<template>
  <div class="changelog-page">
    <section class="changelog-hero">
      <div>
        <p class="hero-eyebrow">PRODUCT UPDATES</p>
        <h1>{{ canManage ? '更新日志管理' : '更新日志' }}</h1>
        <p>{{ canManage ? '系统每天自动汇总并发布管理平台、用户端和网关的功能变更；这里可查看和纠正内容。' : '集中查看平台与网关的最新功能、体验优化和问题修复。' }}</p>
      </div>
      <div class="hero-stat">
        <span>当前显示</span><strong>{{ entries.length }}</strong><em>条记录</em>
      </div>
      <div v-if="canManage" class="auto-sync-status">
        <span></span>
        <div><strong>每日自动更新</strong><small>每天 17:00 汇总发布</small></div>
      </div>
    </section>

    <section class="filter-panel">
      <div class="surface-tabs" aria-label="按产品端筛选">
        <button type="button" :class="{ active: surface === '' }" @click="surface = ''">全部</button>
        <button v-for="item in surfaceOptions" :key="item.value" type="button" :class="{ active: surface === item.value }" @click="surface = item.value">{{ item.label }}</button>
      </div>
      <div class="filter-actions">
        <el-select v-model="category" clearable placeholder="全部类型">
          <el-option v-for="(item, key) in categoryMap" :key="key" :label="item.label" :value="key" />
        </el-select>
        <el-select v-if="canManage" v-model="status" clearable placeholder="全部状态">
          <el-option label="已发布" value="PUBLISHED" /><el-option label="草稿" value="DRAFT" />
        </el-select>
        <el-input v-model="keyword" clearable :prefix-icon="Search" placeholder="搜索版本或更新内容" @keyup.enter="load" @clear="load" />
        <el-button :icon="RefreshRight" :loading="loading" @click="load">刷新</el-button>
      </div>
    </section>

    <section v-loading="loading" class="timeline-panel">
      <div v-if="!groupedEntries.length && !loading" class="empty-state">
        <div class="empty-icon">更</div><strong>暂无符合条件的更新</strong><span>调整筛选条件后再查看</span>
      </div>
      <div v-for="group in groupedEntries" :key="group.date" class="date-group">
        <aside class="date-column"><time>{{ dateLabel(group.date) }}</time><span>{{ group.entries.length }} 项更新</span></aside>
        <div class="entries-column">
          <article v-for="entry in group.entries" :key="entry.id" class="change-card" :class="{ draft: entry.status === 'DRAFT' }">
            <div class="timeline-dot" :class="categoryMap[entry.category].tone"></div>
            <header class="card-head">
              <div class="card-meta">
                <code v-if="entry.version">{{ entry.version }}</code>
                <span class="category-badge" :class="categoryMap[entry.category].tone">{{ categoryMap[entry.category].label }}</span>
                <span v-if="entry.status === 'DRAFT'" class="draft-badge">草稿</span>
              </div>
              <div v-if="canManage" class="card-actions">
                <el-button link type="primary" :icon="Edit" @click="openEdit(entry)">编辑</el-button>
                <el-button link type="danger" :icon="Delete" @click="remove(entry)">删除</el-button>
              </div>
            </header>
            <h2>{{ entry.title }}</h2>
            <p v-if="entry.summary" class="summary">{{ entry.summary }}</p>
            <ul class="change-list">
              <li v-for="(item, index) in entry.items" :key="index"><span><Check /></span><p>{{ item.text }}</p></li>
            </ul>
            <footer>
              <div class="surface-badges"><span v-for="item in entry.surfaces" :key="item">{{ surfaceLabel(item) }}</span></div>
              <small v-if="canManage">由 {{ entry.createdByName }} 记录</small>
            </footer>
          </article>
        </div>
      </div>
    </section>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑更新日志' : '新增更新日志'" width="min(760px, 94vw)" class="changelog-dialog" destroy-on-close>
      <el-form label-position="top">
        <div class="form-grid">
          <el-form-item label="发布日期" required><el-date-picker v-model="form.releaseDate" type="date" value-format="YYYY-MM-DD" placeholder="选择日期" /></el-form-item>
          <el-form-item label="版本号"><el-input v-model="form.version" maxlength="30" placeholder="例如 v1.6.0（可选）" /></el-form-item>
        </div>
        <el-form-item label="更新标题" required><el-input v-model="form.title" maxlength="120" show-word-limit placeholder="概括本次主要更新" /></el-form-item>
        <el-form-item label="简短说明"><el-input v-model="form.summary" maxlength="300" show-word-limit placeholder="补充本次更新的背景或目标（可选）" /></el-form-item>
        <div class="form-grid">
          <el-form-item label="适用端" required>
            <el-checkbox-group v-model="form.surfaces"><el-checkbox v-for="item in surfaceOptions" :key="item.value" :value="item.value" :label="item.value">{{ item.label }}</el-checkbox></el-checkbox-group>
          </el-form-item>
          <el-form-item label="更新类型" required>
            <el-select v-model="form.category"><el-option v-for="(item, key) in categoryMap" :key="key" :label="item.label" :value="key" /></el-select>
          </el-form-item>
        </div>
        <el-form-item label="功能明细" required>
          <div class="item-editor">
            <div v-for="(item, index) in form.items" :key="index" class="item-row">
              <span>{{ index + 1 }}</span><el-input v-model="item.text" maxlength="300" placeholder="说明完成了什么功能或修复了什么问题" />
              <el-button text type="danger" :icon="Delete" aria-label="删除该条" @click="removeItem(index)" />
            </div>
            <el-button class="add-item" plain :icon="Plus" :disabled="form.items.length >= 30" @click="addItem">添加一条</el-button>
          </div>
        </el-form-item>
        <el-form-item label="发布状态"><el-radio-group v-model="form.status"><el-radio value="PUBLISHED">立即发布</el-radio><el-radio value="DRAFT">保存草稿</el-radio></el-radio-group></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="save">{{ form.status === 'PUBLISHED' ? '发布' : '保存草稿' }}</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.changelog-page { max-width: 1400px; margin: 0 auto; }
.changelog-hero { display: grid; grid-template-columns: 1fr auto auto; align-items: center; gap: 24px; margin-bottom: 18px; padding: 26px 28px; border: 1px solid #dce6f2; border-radius: 16px; background: linear-gradient(135deg, #fff 35%, #f0f6ff 100%); box-shadow: 0 10px 32px rgb(36 72 120 / 5%); }
.hero-eyebrow { margin: 0 0 8px; color: #377cf6; font-size: 11px; font-weight: 700; letter-spacing: .14em; }
.changelog-hero h1 { margin: 0; color: #13233a; font-size: 28px; }
.changelog-hero p:not(.hero-eyebrow) { margin: 9px 0 0; color: #73839b; font-size: 14px; }
.hero-stat { display: flex; align-items: baseline; gap: 5px; padding: 8px 18px; border-right: 1px solid #dce6f2; color: #8190a6; font-size: 12px; }
.hero-stat strong { color: #1b62d1; font-size: 26px; }.hero-stat em { font-style: normal; }
.auto-sync-status { display: flex; align-items: center; gap: 10px; padding: 9px 13px; border: 1px solid #cde8dd; border-radius: 10px; color: #176d51; background: #f1fbf7; }
.auto-sync-status > span { width: 9px; height: 9px; border-radius: 50%; background: #22a976; box-shadow: 0 0 0 4px rgb(34 169 118 / 12%); }
.auto-sync-status div { display: grid; gap: 2px; }.auto-sync-status strong { font-size: 13px; }.auto-sync-status small { color: #719688; font-size: 11px; }
.filter-panel { display: flex; align-items: center; justify-content: space-between; gap: 18px; margin-bottom: 20px; padding: 12px 14px; border: 1px solid #e0e8f2; border-radius: 12px; background: #fff; }
.surface-tabs { display: flex; gap: 4px; padding: 3px; border-radius: 9px; background: #f3f6fa; }
.surface-tabs button { min-width: 74px; padding: 8px 14px; border: 0; border-radius: 7px; color: #6d7c91; background: transparent; cursor: pointer; }
.surface-tabs button.active { color: #1868dd; background: #fff; box-shadow: 0 2px 8px rgb(36 72 120 / 10%); font-weight: 600; }
.filter-actions { display: flex; align-items: center; gap: 10px; }.filter-actions :deep(.el-select) { width: 130px; }.filter-actions :deep(.el-input) { width: 240px; }
.timeline-panel { min-height: 260px; padding: 12px 0 40px; }
.date-group { display: grid; grid-template-columns: 185px minmax(0, 1fr); gap: 34px; }
.date-column { position: sticky; top: 86px; align-self: start; display: grid; gap: 7px; padding: 26px 0 0 4px; }
.date-column time { color: #23344d; font-size: 14px; font-weight: 700; }.date-column span { color: #94a1b4; font-size: 12px; }
.entries-column { position: relative; padding: 18px 0 26px 28px; border-left: 1px solid #dce5f0; }
.change-card { position: relative; margin-bottom: 18px; padding: 23px 25px 20px; border: 1px solid #dee7f2; border-radius: 14px; background: #fff; box-shadow: 0 8px 24px rgb(34 61 98 / 5%); }
.change-card:last-child { margin-bottom: 0; }.change-card.draft { border-style: dashed; background: #fbfcfe; }
.timeline-dot { position: absolute; left: -35px; top: 30px; width: 12px; height: 12px; border: 3px solid #f4f7fb; border-radius: 50%; box-shadow: 0 0 0 1px currentColor; }
.timeline-dot.feature { color: #3478f6; background: #3478f6; }.timeline-dot.improvement { color: #11a87d; background: #11a87d; }.timeline-dot.fix { color: #e5a424; background: #e5a424; }.timeline-dot.security { color: #8b65d6; background: #8b65d6; }
.card-head { display: flex; align-items: center; justify-content: space-between; gap: 14px; }.card-meta { display: flex; align-items: center; gap: 8px; }
.card-meta code { color: #1e2f49; font-family: ui-monospace, SFMono-Regular, Consolas, monospace; font-size: 13px; font-weight: 700; }
.category-badge,.draft-badge { padding: 3px 8px; border-radius: 999px; font-size: 11px; }.category-badge.feature { color: #276ce2; background: #eaf2ff; }.category-badge.improvement { color: #078767; background: #e7f8f2; }.category-badge.fix { color: #ad7410; background: #fff5dc; }.category-badge.security { color: #7651bd; background: #f0eaff; }.draft-badge { color: #66758a; background: #eef1f5; }
.card-actions { display: flex; }.change-card h2 { margin: 13px 0 8px; color: #172942; font-size: 19px; }.summary { margin: 0 0 14px; color: #748399; font-size: 13px; }
.change-list { display: grid; gap: 10px; margin: 16px 0 20px; padding: 0; list-style: none; }.change-list li { display: flex; align-items: flex-start; gap: 10px; color: #526279; line-height: 1.65; }.change-list li > span { display: grid; flex: 0 0 18px; height: 18px; margin-top: 3px; place-items: center; border: 1px solid #d2dce8; border-radius: 50%; color: #7590af; }.change-list svg { width: 11px; }.change-list p { margin: 0; }
.change-card footer { display: flex; align-items: center; justify-content: space-between; padding-top: 15px; border-top: 1px solid #edf1f6; }.surface-badges { display: flex; gap: 7px; }.surface-badges span { padding: 4px 9px; border-radius: 5px; color: #52709b; background: #f0f5fb; font-size: 11px; }.change-card footer small { color: #9aa6b7; }
.empty-state { display: grid; place-items: center; gap: 9px; padding: 80px 20px; color: #8a98ac; }.empty-icon { display: grid; width: 48px; height: 48px; place-items: center; border-radius: 14px; color: #4d80ca; background: #edf4ff; font-weight: 700; }.empty-state strong { color: #516078; }.empty-state span { font-size: 12px; }
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }.form-grid :deep(.el-date-editor),.form-grid :deep(.el-select) { width: 100%; }.item-editor { display: grid; width: 100%; gap: 9px; }.item-row { display: grid; grid-template-columns: 26px minmax(0, 1fr) 32px; align-items: center; gap: 7px; }.item-row > span { display: grid; width: 24px; height: 24px; place-items: center; border-radius: 50%; color: #557092; background: #edf3fa; font-size: 11px; }.add-item { justify-self: start; }
@media (max-width: 900px) { .changelog-hero { grid-template-columns: 1fr auto; }.hero-stat { display: none; }.filter-panel { align-items: stretch; flex-direction: column; }.filter-actions { flex-wrap: wrap; }.date-group { grid-template-columns: 1fr; gap: 0; }.date-column { position: static; padding: 20px 0 4px; }.entries-column { margin-left: 7px; }.filter-actions :deep(.el-input) { flex: 1; width: 210px; } }
@media (max-width: 600px) { .changelog-hero { grid-template-columns: 1fr; }.changelog-hero .el-button { width: 100%; }.surface-tabs { display: grid; grid-template-columns: repeat(4, 1fr); }.surface-tabs button { min-width: 0; padding-inline: 7px; }.filter-actions > * { flex: 1 1 100%; width: 100% !important; }.entries-column { padding-left: 20px; }.change-card { padding: 19px 17px; }.timeline-dot { left: -27px; }.form-grid { grid-template-columns: 1fr; gap: 0; } }
</style>
