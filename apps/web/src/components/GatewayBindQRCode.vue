<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import QRCode from 'qrcode';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Download, Printer } from '@element-plus/icons-vue';
import { getLocalPrintAgentSettings, printWithLocalAgent } from '../utils/local-print-agent';

const props = withDefaults(defineProps<{
  sn: string;
  bindCode: string;
  compact?: boolean;
}>(), { compact: false });

const dataURL = ref('');
const printing = ref(false);
const savedLabelSize = localStorage.getItem('gateway-label-size');
// 50x30 was the legacy default. Existing browsers may still retain it even
// after the production label was changed to 70x50, so migrate that default.
const labelSize = ref(savedLabelSize === '50x30' ? '70x50' : (savedLabelSize || '70x50'));
const printCopies = ref(Number(localStorage.getItem('gateway-label-copies') || 1));
const labelSizes = [
  { value: '50x30', label: '50 × 30 mm', width: 50, height: 30 },
  { value: '60x40', label: '60 × 40 mm', width: 60, height: 40 },
  { value: '70x50', label: '70 × 50 mm', width: 70, height: 50 },
];
const selectedLabelSize = computed(() => labelSizes.find((item) => item.value === labelSize.value) || labelSizes[2]);
const previewStyle = computed(() => ({ aspectRatio: `${selectedLabelSize.value.width} / ${selectedLabelSize.value.height}` }));
const payload = computed(() => JSON.stringify({
  type: 'weikong-gateway-bind',
  version: 1,
  sn: props.sn.trim().toUpperCase(),
  bindCode: props.bindCode.trim(),
}));

watch(payload, async (value) => {
  dataURL.value = await QRCode.toDataURL(value, {
    errorCorrectionLevel: 'M',
    margin: 2,
    width: 360,
    color: { dark: '#0f172a', light: '#ffffff' },
  });
}, { immediate: true });

function download() {
  if (!dataURL.value) return;
  const link = document.createElement('a');
  link.href = dataURL.value;
  link.download = `gateway-bind-${props.sn}.png`;
  link.click();
}

function escapeHtml(value: string) {
  return value.replace(/[&<>'"]/g, (char) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;',
  })[char] || char);
}

async function printLabel() {
  if (!dataURL.value || printing.value) return;

  const localAgent = getLocalPrintAgentSettings();
  if (!localAgent.enabled) {
    try {
      await ElMessageBox.confirm(
        `<div class="label-print-confirm">
          <div class="label-print-preview">
            <img src="${dataURL.value}" alt="绑定二维码" />
            <div><b>${escapeHtml(props.sn)}</b><span>绑定码 ${escapeHtml(props.bindCode)}</span></div>
          </div>
          <p>打印时请选择 <b>Godex G500 GZPL</b>，并确认标签纸规格与打印机中实际安装的标签一致。</p>
        </div>`,
        '打印网关绑定标签',
        {
          confirmButtonText: '进入打印',
          cancelButtonText: '取消',
          dangerouslyUseHTMLString: true,
          customClass: 'gateway-label-print-confirm',
        },
      );
    } catch {
      return;
    }
  }

  const size = labelSizes.find((item) => item.value === labelSize.value) || labelSizes[0];
  localStorage.setItem('gateway-label-size', size.value);
  localStorage.setItem('gateway-label-copies', String(printCopies.value));
  printing.value = true;
  await nextTick();

  try {
    if (await printWithLocalAgent([{ sn: props.sn, bindCode: props.bindCode }], size.value, printCopies.value)) {
      printing.value = false;
      ElMessage.success('标签已发送到本地打印机');
      return;
    }
  } catch (error) {
    printing.value = false;
    ElMessage.error(error instanceof Error ? `本地打印失败：${error.message}` : '本地打印失败');
    return;
  }

  const printWindow = window.open('', '_blank', 'popup,width=720,height=560');
  if (!printWindow) {
    printing.value = false;
    ElMessage.error('浏览器阻止了打印窗口，请允许本站弹出窗口后重试');
    return;
  }

  const copies = Math.max(1, Math.min(20, Number(printCopies.value) || 1));
  const labels = Array.from({ length: copies }, () => `
    <section class="label">
      <img src="${dataURL.value}" alt="网关绑定二维码" />
      <div class="label-info">
        <div class="brand">微控网关</div>
        <div class="caption">扫码绑定设备</div>
        <div class="sn">${escapeHtml(props.sn.trim().toUpperCase())}</div>
        <div class="bind-code"><span>绑定码</span><strong>${escapeHtml(props.bindCode.trim())}</strong></div>
      </div>
    </section>
  `).join('');

  printWindow.document.open();
  printWindow.document.write(`<!doctype html>
    <html lang="zh-CN"><head><meta charset="utf-8"><title>打印网关标签 - ${escapeHtml(props.sn)}</title>
    <style>
      @page { size: ${size.width}mm ${size.height}mm; margin: 0; }
      * { box-sizing: border-box; }
      html, body { margin: 0; padding: 0; background: #fff; color: #111827; font-family: Arial, "Microsoft YaHei", sans-serif; }
      .label { width: ${size.width}mm; height: ${size.height}mm; padding: 2mm; display: flex; align-items: center; gap: 1.6mm; overflow: hidden; page-break-after: always; break-after: page; }
      .label:last-child { page-break-after: auto; break-after: auto; }
      .label > img { width: ${Math.max(22, size.height - 4)}mm; height: ${Math.max(22, size.height - 4)}mm; flex: 0 0 auto; image-rendering: crisp-edges; }
      .label-info { min-width: 0; flex: 1; display: flex; flex-direction: column; justify-content: center; line-height: 1.15; }
      .brand { font-size: ${size.height >= 40 ? 12 : 9}pt; font-weight: 800; }
      .caption { margin-top: 0.7mm; color: #64748b; font-size: ${size.height >= 40 ? 7 : 6}pt; }
      .sn { margin-top: 1.4mm; font-family: Consolas, monospace; font-size: ${size.height >= 40 ? 9 : 7}pt; font-weight: 700; overflow-wrap: anywhere; }
      .bind-code { margin-top: 1.4mm; display: flex; flex-direction: column; gap: 0.4mm; }
      .bind-code span { color: #64748b; font-size: 6pt; }
      .bind-code strong { font-family: Consolas, monospace; font-size: ${size.height >= 40 ? 16 : 12}pt; letter-spacing: 0.8mm; }
      @media screen { body { background: #eef2f7; padding: 20px; } .label { margin: 0 auto 16px; background: #fff; box-shadow: 0 8px 24px rgba(15,23,42,.14); } }
      @media print { html, body { width: ${size.width}mm; } }
    </style></head><body>${labels}<script>
      window.onload = function () { setTimeout(function () { window.print(); }, 120); };
      window.onafterprint = function () { window.close(); };
    <\/script></body></html>`);
  printWindow.document.close();
  printing.value = false;
}

async function copyPayload() {
  await navigator.clipboard.writeText(payload.value);
  ElMessage.success('绑定二维码内容已复制');
}
</script>

<template>
  <div class="bind-qr" :class="{ compact }">
    <div class="label-preview-wrap">
      <div class="label-preview" :style="previewStyle">
        <img v-if="dataURL" :src="dataURL" :alt="`${sn} 绑定二维码`" />
        <div class="label-preview-copy">
          <b>WEIKONG</b>
          <small>SCAN TO BIND</small>
          <span>SN</span>
          <code>{{ sn.trim().toUpperCase() }}</code>
          <span>BIND CODE</span>
          <strong>{{ bindCode.trim() }}</strong>
        </div>
      </div>
      <em>{{ selectedLabelSize.label }} 打印预览</em>
    </div>
    <div class="qr-content">
      <strong>App 扫码绑定</strong>
      <p>交付客户后，在物联云 App 的设备页点击“＋ → 扫码绑定网关”。</p>
      <div class="qr-actions">
        <el-button type="primary" :icon="Printer" :loading="printing" @click="printLabel">打印标签</el-button>
        <el-button :icon="Download" @click="download">下载二维码</el-button>
        <el-button @click="copyPayload">复制二维码内容</el-button>
      </div>
      <div class="print-options">
        <span>标签纸</span>
        <el-select v-model="labelSize" size="small" style="width: 126px">
          <el-option v-for="item in labelSizes" :key="item.value" :label="item.label" :value="item.value" />
        </el-select>
        <span>份数</span>
        <el-input-number v-model="printCopies" size="small" :min="1" :max="20" controls-position="right" />
      </div>
      <small>预览与 GZPL 打印采用相同布局。请选择与打印机内实际标签纸一致的规格；二维码包含 SN 和绑定码。</small>
    </div>
  </div>
</template>

<style scoped>
.bind-qr { display: grid; grid-template-columns:minmax(280px, 360px) minmax(0, 1fr); gap: 18px; align-items: center; margin-top: 16px; padding: 16px; border: 1px solid #dbeafe; border-radius: 12px; background: #f8fbff; }
.label-preview-wrap { display:grid; justify-items:center; gap:8px; min-width:0; }
.label-preview-wrap em { color:#64748b; font-size:11px; font-style:normal; }
.label-preview { display:grid; grid-template-columns:46% minmax(0,1fr); gap:4%; align-items:center; width:100%; max-width:360px; padding:4%; overflow:hidden; border:1px solid #cbd5e1; border-radius:8px; background:#fff; box-shadow:0 9px 25px rgba(15,23,42,.12); }
.label-preview>img { display:block; width:100%; aspect-ratio:1; object-fit:contain; image-rendering:pixelated; }
.label-preview-copy { display:flex; min-width:0; flex-direction:column; align-items:flex-start; line-height:1.05; }
.label-preview-copy b { color:#111827; font:800 clamp(12px,1.2vw,18px)/1 Arial,sans-serif; }
.label-preview-copy small { margin-top:5px; color:#64748b; font:600 clamp(6px,.55vw,9px)/1 Arial,sans-serif; }
.label-preview-copy span { margin-top:10px; color:#64748b; font:600 clamp(6px,.6vw,9px)/1 Arial,sans-serif; }
.label-preview-copy code { display:block; width:100%; margin-top:4px; overflow-wrap:anywhere; color:#111827; font:700 clamp(7px,.65vw,10px)/1.15 Consolas,monospace; }
.label-preview-copy strong { margin-top:4px; color:#111827; font:800 clamp(13px,1.35vw,21px)/1 Consolas,monospace; letter-spacing:.08em; }
.qr-content { min-width: 0; }
.qr-content strong { color: #172033; font-size: 15px; }
.qr-content p { margin: 6px 0 12px; color: #64748b; font-size: 13px; line-height: 1.55; }
.qr-actions { display: flex; flex-wrap: wrap; gap: 8px; }
.qr-actions .el-button { margin-left: 0; }
.print-options { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; margin-top: 10px; color: #64748b; font-size: 12px; }
.print-options :deep(.el-input-number) { width: 92px; }
.qr-content small { display: block; margin-top: 11px; color: #94a3b8; font-size: 11px; line-height: 1.5; }
.bind-qr.compact { grid-template-columns:1fr; justify-items:stretch; }
.bind-qr.compact .label-preview-wrap { width:100%; }
.bind-qr.compact .label-preview { max-width:320px; }
.bind-qr.compact .qr-content { width:100%; }
@media (max-width: 640px) {
  .bind-qr, .bind-qr.compact { grid-template-columns: 1fr; justify-items: stretch; text-align: center; }
  .qr-actions { justify-content: center; }
}
</style>

<style>
.gateway-label-print-confirm { width: min(460px, 90vw); border-radius: 14px; }
.label-print-confirm { padding-top: 4px; }
.label-print-confirm p { margin: 14px 0 0; color: #64748b; font-size: 13px; line-height: 1.6; }
.label-print-preview { display: flex; align-items: center; gap: 14px; padding: 12px; border: 1px solid #dbeafe; border-radius: 10px; background: #f8fbff; }
.label-print-preview img { width: 82px; height: 82px; border-radius: 6px; background: #fff; }
.label-print-preview div { min-width: 0; display: flex; flex-direction: column; gap: 7px; }
.label-print-preview b { color: #172033; font-family: Consolas, monospace; font-size: 15px; overflow-wrap: anywhere; }
.label-print-preview span { color: #475569; font-size: 13px; }
</style>
