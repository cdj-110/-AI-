<script setup lang="ts">
import { computed, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { Link, Monitor, RefreshRight, View } from '@element-plus/icons-vue';

const defaultGoviewUrl = (import.meta.env.VITE_GOVIEW_URL as string | undefined) ?? 'http://127.0.0.1:2024';
const storedGoviewUrl = localStorage.getItem('goviewUrl');
const savedGoviewUrl = storedGoviewUrl && storedGoviewUrl !== 'http://127.0.0.1:3020' ? storedGoviewUrl : defaultGoviewUrl;
const goviewUrl = ref(savedGoviewUrl);
const draftUrl = ref(savedGoviewUrl);

const normalizedGoviewUrl = computed(() => goviewUrl.value.trim().replace(/\/$/, ''));
function buildGoviewRoute(path: string) {
  return normalizedGoviewUrl.value ? `${normalizedGoviewUrl.value}/#${path}` : '';
}

const designerUrl = computed(() => buildGoviewRoute('/largeScreen'));
const previewUrl = computed(() => buildGoviewRoute('/largeScreen'));

const integrationSteps = [
  'GoView 已作为 apps/goview-web 子应用合入当前仓库，开发环境默认运行在 2024 端口。',
  '在这里填写 GoView 访问地址，例如 http://192.168.1.128:2024。',
  'GoView 页面通过接口或 MQTT/WebSocket 数据源读取本平台的设备实时数据。',
  '后续可把本平台 token 透传给 GoView，实现单点登录和租户隔离。',
];

function saveUrl() {
  const next = draftUrl.value.trim().replace(/\/$/, '');
  goviewUrl.value = next;
  if (next) {
    localStorage.setItem('goviewUrl', next);
    ElMessage.success('GoView 地址已保存');
  } else {
    localStorage.removeItem('goviewUrl');
    ElMessage.info('已清空 GoView 地址');
  }
}

function openDesigner() {
  if (!designerUrl.value) {
    ElMessage.warning('请先填写 GoView 访问地址');
    return;
  }
  window.open(designerUrl.value, '_blank');
}

function openPreview() {
  if (!previewUrl.value) {
    ElMessage.warning('请先填写 GoView 访问地址');
    return;
  }
  window.open(previewUrl.value, '_blank');
}

function reloadFrame() {
  const current = goviewUrl.value;
  goviewUrl.value = '';
  requestAnimationFrame(() => {
    goviewUrl.value = current;
  });
}
</script>

<template>
  <div class="cloud-scada-page">
    <section class="hero-card">
      <div>
        <p class="eyebrow">CLOUD SCADA</p>
        <h1>云组态</h1>
        <p>接入 GoView 作为可视化组态设计器，用于搭建设备看板、生产大屏和实时监控页面。</p>
      </div>
      <div class="hero-actions">
        <el-button :icon="View" @click="openPreview">运行预览</el-button>
        <el-button type="primary" :icon="Monitor" @click="openDesigner">打开设计器</el-button>
      </div>
    </section>

    <section class="config-card">
      <div class="section-title">
        <div>
          <h2>GoView 接入地址</h2>
          <p>GoView 建议独立部署，本平台先通过内嵌页面承载；后续再补充项目同步、权限和数据源绑定。</p>
        </div>
        <el-tag :type="normalizedGoviewUrl ? 'success' : 'info'">{{ normalizedGoviewUrl ? '已配置' : '未配置' }}</el-tag>
      </div>
      <div class="url-row">
        <el-input v-model="draftUrl" placeholder="例如 http://192.168.1.128:2024" clearable>
          <template #prepend>GoView URL</template>
        </el-input>
        <el-button type="primary" :icon="Link" @click="saveUrl">保存地址</el-button>
        <el-button :icon="RefreshRight" @click="reloadFrame">刷新内嵌页</el-button>
      </div>
    </section>

    <section class="content-grid">
      <article class="panel">
        <div class="section-title compact">
          <div>
            <h2>内嵌设计器</h2>
            <p>配置 GoView 地址后会在这里显示设计器页面。</p>
          </div>
        </div>
        <div v-if="designerUrl" class="iframe-shell">
          <iframe :src="designerUrl" title="GoView 设计器" />
        </div>
        <div v-else class="empty-state">
          <strong>还没有配置 GoView 地址</strong>
          <p>先部署 GoView，然后在上方填写访问地址即可接入。</p>
        </div>
      </article>

      <aside class="panel guide-panel">
        <h2>接入建议</h2>
        <ol>
          <li v-for="step in integrationSteps" :key="step">{{ step }}</li>
        </ol>
        <div class="data-source">
          <h3>推荐数据源</h3>
          <p>设备列表：<code>/api/devices</code></p>
          <p>设备详情：<code>/api/devices/:id</code></p>
          <p>实时遥测：<code>/api/devices/:id/telemetry/stream</code></p>
          <p>最近上报：<code>/api/devices/:id/telemetry</code></p>
        </div>
      </aside>
    </section>
  </div>
</template>

<style scoped>
.cloud-scada-page { display: grid; gap: 18px; }
.hero-card, .config-card, .panel { border: 1px solid #e9eef5; border-radius: 18px; background: #fff; box-shadow: 0 14px 34px rgb(15 23 42 / 5%); }
.hero-card { display: flex; align-items: center; justify-content: space-between; gap: 18px; padding: 24px; background: radial-gradient(circle at top right, rgb(37 99 235 / 12%), transparent 32%), #fff; }
.hero-card h1 { margin: 4px 0 8px; color: #172033; font-size: 28px; }
.hero-card p { margin: 0; color: #64748b; line-height: 1.7; }
.eyebrow { margin: 0; color: #2563eb; font-size: 12px; font-weight: 800; letter-spacing: 1.6px; }
.hero-actions, .url-row { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; }
.config-card, .panel { padding: 20px; }
.section-title { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 14px; }
.section-title.compact { margin-bottom: 12px; }
.section-title h2, .guide-panel h2 { margin: 0; color: #172033; font-size: 18px; }
.section-title p { margin: 6px 0 0; color: #697386; font-size: 13px; }
.url-row .el-input { flex: 1; min-width: 280px; }
.content-grid { display: grid; grid-template-columns: minmax(0, 1fr) 340px; gap: 18px; align-items: start; }
.iframe-shell { overflow: hidden; height: calc(100vh - 310px); min-height: 520px; border: 1px solid #e5eaf2; border-radius: 14px; background: #f8fafc; }
.iframe-shell iframe { width: 100%; height: 100%; border: 0; }
.empty-state { display: grid; min-height: 520px; place-items: center; align-content: center; gap: 8px; border: 1px dashed #cbd5e1; border-radius: 14px; color: #697386; background: #f8fafc; text-align: center; }
.empty-state strong { color: #172033; font-size: 17px; }
.empty-state p { margin: 0; }
.guide-panel ol { margin: 12px 0 0; padding-left: 20px; color: #475569; line-height: 1.8; }
.data-source { margin-top: 18px; padding: 14px; border-radius: 14px; background: #f8fafc; }
.data-source h3 { margin: 0 0 10px; font-size: 15px; }
.data-source p { margin: 8px 0; color: #64748b; font-size: 13px; }
.data-source code { color: #2563eb; font-family: Consolas, monospace; }
@media (max-width: 1100px) {
  .content-grid { grid-template-columns: 1fr; }
  .hero-card { align-items: flex-start; flex-direction: column; }
}
</style>
