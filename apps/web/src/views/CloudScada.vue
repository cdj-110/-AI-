<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { CopyDocument, Delete, EditPen, MoreFilled, Plus, Search, View } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox, type FormInstance, type FormRules, type InputInstance } from 'element-plus';
import { apiRequest } from '../api/request';
import { useUserStore } from '../stores/user';
import { getAuthItem } from '../utils/auth-storage';

interface ScadaProject {
  id: string;
  name: string;
  description?: string | null;
  status: 'DRAFT' | 'PUBLISHED';
  thumbnail?: string | null;
  createdAt: string;
  updatedAt: string;
}

const loading = ref(false);
const saving = ref(false);
const keyword = ref('');
const projects = ref<ScadaProject[]>([]);
const dialogVisible = ref(false);
const editingId = ref('');
const formRef = ref<FormInstance>();
const projectNameInputRef = ref<InputInstance>();
const form = reactive({ name: '', description: '' });
const rules: FormRules = { name: [{ required: true, message: '请输入项目名称', trigger: 'blur' }] };
const dialogTitle = computed(() => editingId.value ? '编辑云组态项目' : '新建云组态项目');
const userStore = useUserStore();

async function loadProjects() {
  loading.value = true;
  try {
    projects.value = await apiRequest<ScadaProject[]>({
      url: '/api/scada-projects', method: 'GET', params: { keyword: keyword.value.trim() },
    });
  } finally { loading.value = false; }
}

function openCreate() {
  editingId.value = '';
  Object.assign(form, { name: '', description: '' });
  dialogVisible.value = true;
}
function openEdit(project: ScadaProject) {
  editingId.value = project.id;
  Object.assign(form, { name: project.name, description: project.description ?? '' });
  dialogVisible.value = true;
}
function handleProjectEnter(event: KeyboardEvent) {
  if (event.key !== 'Enter' || event.isComposing || event.keyCode === 229) return;
  // 项目说明保留回车换行；Ctrl / Command + Enter 可提交。
  if ((event.target as HTMLElement | null)?.tagName === 'TEXTAREA' && !event.ctrlKey && !event.metaKey) return;
  event.preventDefault();
  event.stopPropagation();
  if (!event.repeat) void saveProject();
}
async function saveProject() {
  if (saving.value) return;
  saving.value = true;
  try {
    if (!await formRef.value?.validate().catch(() => false)) return;
    if (editingId.value) {
      await apiRequest({ url: `/api/scada-projects/${editingId.value}`, method: 'PATCH', data: form });
      ElMessage.success('项目信息已更新');
    } else {
      await apiRequest({ url: '/api/scada-projects', method: 'POST', data: form });
      ElMessage.success('云组态项目已创建');
    }
    dialogVisible.value = false;
    await loadProjects();
  } finally { saving.value = false; }
}
function goViewBaseURL() {
  const configured = String(import.meta.env.VITE_GOVIEW_DESIGNER_URL || '').trim();
  if (configured) return configured.replace(/\/$/, '');
  const port = import.meta.env.DEV ? '3020' : '8080';
  return `${window.location.protocol}//${window.location.hostname}:${port}`;
}

function platformApiBaseURL() {
  const configured = String(import.meta.env.VITE_API_BASE_URL || '').trim();
  const root = (/^https?:\/\//i.test(configured) ? configured : `${window.location.origin}${configured}`).replace(/\/$/, '');
  return root.endsWith('/api') ? root : `${root}/api`;
}

function goViewURL(project: ScadaProject, mode: 'design' | 'preview', nonce: string) {
  const query = new URLSearchParams({
    auth_bridge: '1',
    auth_nonce: nonce,
    platform_origin: window.location.origin,
  });
  const route = mode === 'design'
    ? `/chart/home/${project.id}`
    : `/chart/preview/${project.id}/${encodeURIComponent('画布1')}`;
  return `${goViewBaseURL()}/#${route}?${query.toString()}`;
}

function openGoView(project: ScadaProject, mode: 'design' | 'preview') {
  const token = getAuthItem('accessToken') || userStore.token;
  const userId = userStore.userInfo?.id;
  if (!token || !userId) {
    ElMessage.error('登录状态已失效，请重新登录后再打开 GoView');
    return;
  }

  const nonce = crypto.randomUUID();
  const url = goViewURL(project, mode, nonce);
  const targetOrigin = new URL(goViewBaseURL(), window.location.origin).origin;
  let child: Window | null = null;

  const cleanup = () => {
    window.removeEventListener('message', handleReady);
    window.clearTimeout(timeout);
  };
  const handleReady = (event: MessageEvent) => {
    if (!child || event.source !== child || event.origin !== targetOrigin) return;
    if (event.data?.type !== 'wk-auth-ready' || event.data?.nonce !== nonce) return;
    child.postMessage({
      type: 'wk-platform-auth',
      nonce,
      token,
      user_id: userId,
      config_id: project.id,
      config_name: project.name,
      api_base: platformApiBaseURL(),
      platform_origin: window.location.origin,
      tenant_id: userStore.userInfo?.tenantId,
    }, targetOrigin);
    cleanup();
  };
  const timeout = window.setTimeout(cleanup, 15_000);
  window.addEventListener('message', handleReady);
  child = window.open(url, '_blank');
  if (!child) {
    cleanup();
    ElMessage.warning('浏览器阻止了新窗口，请允许弹出窗口后重试');
  }
}

function openDesigner(project: ScadaProject) { openGoView(project, 'design'); }
function openPreview(project: ScadaProject) { openGoView(project, 'preview'); }
async function duplicateProject(project: ScadaProject) {
  await apiRequest({ url: `/api/scada-projects/${project.id}/duplicate`, method: 'POST' });
  ElMessage.success('项目已复制');
  await loadProjects();
}
async function removeProject(project: ScadaProject) {
  await ElMessageBox.confirm(`删除“${project.name}”后无法恢复，确定继续吗？`, '删除云组态项目', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' });
  await apiRequest({ url: `/api/scada-projects/${project.id}`, method: 'DELETE' });
  ElMessage.success('项目已删除');
  await loadProjects();
}
function formatTime(value: string) { return new Date(value).toLocaleString('zh-CN', { hour12: false }); }
onMounted(loadProjects);
</script>

<template>
  <div class="scada-page">
    <header class="page-heading">
      <div><h1>云组态</h1><p>创建可视化监控项目，在 GoView 设计器中拖拽图表并绑定设备实时点位。</p></div>
      <el-button type="primary" :icon="Plus" @click="openCreate">新建项目</el-button>
    </header>
    <section class="toolbar-card">
      <el-input v-model="keyword" :prefix-icon="Search" clearable placeholder="搜索项目名称" @keyup.enter="loadProjects" @clear="loadProjects" />
      <el-button @click="loadProjects">查询</el-button><span class="project-total">共 {{ projects.length }} 个项目</span>
    </section>
    <div v-loading="loading" class="project-grid">
      <button class="create-card" type="button" @click="openCreate">
        <span class="create-icon"><el-icon><Plus /></el-icon></span><strong>新建云组态项目</strong><small>从空白画布开始设计</small>
      </button>
      <article v-for="project in projects" :key="project.id" class="project-card">
        <div class="preview-shell" @click="openPreview(project)">
          <img v-if="project.thumbnail" :src="project.thumbnail" :alt="`${project.name}预览图`" />
          <div v-else class="default-preview">
            <div class="preview-title">{{ project.name }}</div><div class="preview-kpis"><i /><i /><i /></div>
            <div class="preview-chart"><span v-for="height in [34, 58, 43, 75, 52, 84, 64]" :key="height" :style="{ height: `${height}%` }" /></div>
          </div>
          <span class="preview-action"><el-icon><View /></el-icon> 预览</span>
        </div>
        <div class="project-info">
          <div class="title-row"><strong :title="project.name">{{ project.name }}</strong><el-tag size="small" :type="project.status === 'PUBLISHED' ? 'success' : 'info'">{{ project.status === 'PUBLISHED' ? '已发布' : '草稿' }}</el-tag></div>
          <p>{{ project.description || '暂无项目说明' }}</p><small>更新于 {{ formatTime(project.updatedAt) }}</small>
        </div>
        <div class="card-actions">
          <button type="button" @click="openPreview(project)"><el-icon><View /></el-icon>预览</button>
          <button type="button" @click="openEdit(project)"><el-icon><EditPen /></el-icon>编辑</button>
          <button class="design-button" type="button" @click="openDesigner(project)">设计</button>
          <el-dropdown trigger="click">
            <button class="more-button" type="button" aria-label="更多操作"><el-icon><MoreFilled /></el-icon></button>
            <template #dropdown><el-dropdown-menu><el-dropdown-item :icon="CopyDocument" @click="duplicateProject(project)">复制项目</el-dropdown-item><el-dropdown-item :icon="Delete" divided @click="removeProject(project)">删除项目</el-dropdown-item></el-dropdown-menu></template>
          </el-dropdown>
        </div>
      </article>
    </div>
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="520px" destroy-on-close @opened="projectNameInputRef?.focus()" @keydown.capture="handleProjectEnter">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent="saveProject">
        <el-form-item label="项目名称" prop="name"><el-input ref="projectNameInputRef" v-model="form.name" maxlength="80" show-word-limit placeholder="例如：一号车间生产看板" /></el-form-item>
        <el-form-item label="项目说明"><el-input v-model="form.description" type="textarea" :rows="3" maxlength="300" show-word-limit placeholder="说明这个大屏的使用场景" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="saveProject">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.scada-page { display: grid; gap: 18px; }
.page-heading { display: flex; align-items: center; justify-content: space-between; gap: 20px; }
.page-heading h1 { margin: 0; color: #111827; font-size: 28px; }
.page-heading p { margin: 6px 0 0; color: #7b879a; font-size: 14px; }
.toolbar-card { display: flex; align-items: center; gap: 10px; padding: 14px 16px; border: 1px solid #e7edf5; border-radius: 14px; background: #fff; }
.toolbar-card .el-input { width: min(360px, 100%); }.project-total { margin-left: auto; color: #7b879a; font-size: 13px; }
.project-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(310px, 1fr)); align-items: stretch; gap: 18px; min-height: 260px; }
.project-card, .create-card { min-height: 340px; border: 1px solid #e4eaf2; border-radius: 14px; background: #fff; box-shadow: 0 10px 30px rgb(15 23 42 / 5%); transition: transform .18s ease, box-shadow .18s ease, border-color .18s ease; }
.project-card:hover, .create-card:hover { transform: translateY(-2px); border-color: #b9d5ff; box-shadow: 0 14px 34px rgb(37 99 235 / 10%); }
.create-card { display: grid; place-items: center; align-content: center; gap: 10px; color: #64748b; border-style: dashed; cursor: pointer; font: inherit; }.create-card strong { color: #25324a; font-size: 16px; }.create-card small { color: #94a3b8; }
.create-icon { display: grid; width: 52px; height: 52px; place-items: center; border-radius: 16px; color: #fff; background: linear-gradient(135deg, #38a7ff, #246bfe); font-size: 24px; box-shadow: 0 10px 22px rgb(37 112 255 / 25%); }
.project-card { overflow: hidden; }.preview-shell { position: relative; overflow: hidden; height: 190px; cursor: pointer; background: #071225; }.preview-shell img { width: 100%; height: 100%; object-fit: cover; }
.default-preview { height: 100%; padding: 20px; color: #dbeafe; background: radial-gradient(circle at 72% 24%, rgb(28 164 255 / 30%), transparent 30%), linear-gradient(145deg, #071225, #0d2851); }.preview-title { font-size: 15px; font-weight: 700; }
.preview-kpis { display: flex; gap: 8px; margin-top: 16px; }.preview-kpis i { width: 26%; height: 28px; border: 1px solid rgb(93 173 255 / 35%); border-radius: 4px; background: rgb(30 104 185 / 18%); }
.preview-chart { display: flex; align-items: flex-end; gap: 9px; height: 82px; margin-top: 15px; padding: 8px 12px 0; border-bottom: 1px solid rgb(96 165 250 / 30%); }.preview-chart span { flex: 1; border-radius: 3px 3px 0 0; background: linear-gradient(#38bdf8, #2563eb); opacity: .82; }
.preview-action { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; gap: 6px; color: #fff; background: rgb(3 12 29 / 60%); opacity: 0; transition: opacity .18s ease; }.preview-shell:hover .preview-action { opacity: 1; }
.project-info { min-height: 102px; padding: 16px 16px 12px; }.title-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; }.title-row strong { overflow: hidden; color: #172033; font-size: 16px; text-overflow: ellipsis; white-space: nowrap; }
.project-info p { overflow: hidden; margin: 9px 0 8px; color: #667085; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }.project-info small { color: #98a2b3; }
.card-actions { display: grid; grid-template-columns: 1fr 1fr 1fr 44px; height: 46px; border-top: 1px solid #edf1f6; }.card-actions button { display: inline-flex; align-items: center; justify-content: center; gap: 5px; border: 0; border-right: 1px solid #edf1f6; color: #526072; background: #fff; cursor: pointer; font: inherit; font-size: 13px; }.card-actions button:hover { color: #1677ff; background: #f5f9ff; }.card-actions .design-button { color: #1677ff; font-weight: 600; }.more-button { width: 44px; height: 46px; border-right: 0 !important; }
@media (max-width: 720px) { .page-heading { align-items: flex-start; flex-direction: column; }.toolbar-card { flex-wrap: wrap; }.project-total { width: 100%; margin-left: 0; } }
</style>
