import QRCode from 'qrcode';

export const gatewayLabelSizes = [
  { value: '50x30', label: '50 × 30 mm', width: 50, height: 30 },
  { value: '60x40', label: '60 × 40 mm', width: 60, height: 40 },
  { value: '70x50', label: '70 × 50 mm', width: 70, height: 50 },
] as const;

export interface GatewayLabelData {
  sn: string;
  bindCode: string;
}

export interface GatewayLabelPrintOptions {
  size: string;
  copies: number;
  targetWindow?: Window | null;
}

function escapeHtml(value: string) {
  return value.replace(/[&<>'"]/g, (character) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;',
  })[character] || character);
}

export function gatewayBindPayload(item: GatewayLabelData) {
  return JSON.stringify({
    type: 'weikong-gateway-bind',
    version: 1,
    sn: item.sn.trim().toUpperCase(),
    bindCode: item.bindCode.trim(),
  });
}

export async function createGatewayQrDataUrl(item: GatewayLabelData, width = 360) {
  return QRCode.toDataURL(gatewayBindPayload(item), {
    errorCorrectionLevel: 'M',
    margin: 2,
    width,
    color: { dark: '#0f172a', light: '#ffffff' },
  });
}

export async function printGatewayLabels(items: GatewayLabelData[], options: GatewayLabelPrintOptions) {
  const size = gatewayLabelSizes.find((item) => item.value === options.size) || gatewayLabelSizes[0];
  const copies = Math.max(1, Math.min(20, Number(options.copies) || 1));
  const printWindow = options.targetWindow || window.open('', '_blank', 'popup,width=720,height=560');
  if (!printWindow) throw new Error('浏览器阻止了打印窗口，请允许本站弹出窗口后重试');

  printWindow.document.open();
  printWindow.document.write('<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><title>正在生成标签…</title></head><body style="font-family:sans-serif;padding:32px">正在生成二维码标签，请稍候…</body></html>');
  printWindow.document.close();

  const rendered = await Promise.all(items.map(async (item) => ({
    ...item,
    dataUrl: await createGatewayQrDataUrl(item),
  })));
  const labels = rendered.flatMap((item) => Array.from({ length: copies }, () => `
    <section class="label">
      <img src="${item.dataUrl}" alt="网关绑定二维码" />
      <div class="label-info">
        <div class="brand">微控网关</div>
        <div class="caption">扫码绑定设备</div>
        <div class="sn">${escapeHtml(item.sn.trim().toUpperCase())}</div>
        <div class="bind-code"><span>绑定码</span><strong>${escapeHtml(item.bindCode.trim())}</strong></div>
      </div>
    </section>
  `)).join('');

  printWindow.document.open();
  printWindow.document.write(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><title>打印网关标签</title>
    <style>
      @page { size: ${size.width}mm ${size.height}mm; margin: 0; }
      * { box-sizing: border-box; }
      html, body { margin: 0; padding: 0; background: #fff; color: #111827; font-family: Arial, "Microsoft YaHei", sans-serif; }
      .label { width: ${size.width}mm; height: ${size.height}mm; padding: 2mm; display: flex; align-items: center; gap: 1.6mm; overflow: hidden; page-break-after: always; break-after: page; }
      .label:last-child { page-break-after: auto; break-after: auto; }
      .label > img { width: ${Math.max(22, size.height - 4)}mm; height: ${Math.max(22, size.height - 4)}mm; flex: 0 0 auto; image-rendering: crisp-edges; }
      .label-info { min-width: 0; flex: 1; display: flex; flex-direction: column; justify-content: center; line-height: 1.15; }
      .brand { font-size: ${size.height >= 40 ? 12 : 9}pt; font-weight: 800; }
      .caption { margin-top: .7mm; color: #64748b; font-size: ${size.height >= 40 ? 7 : 6}pt; }
      .sn { margin-top: 1.4mm; font-family: Consolas, monospace; font-size: ${size.height >= 40 ? 9 : 7}pt; font-weight: 700; overflow-wrap: anywhere; }
      .bind-code { margin-top: 1.4mm; display: flex; flex-direction: column; gap: .4mm; }
      .bind-code span { color: #64748b; font-size: 6pt; }
      .bind-code strong { font-family: Consolas, monospace; font-size: ${size.height >= 40 ? 16 : 12}pt; letter-spacing: .8mm; }
      @media screen { body { background: #eef2f7; padding: 20px; } .label { margin: 0 auto 16px; background: #fff; box-shadow: 0 8px 24px rgba(15,23,42,.14); } }
      @media print { html, body { width: ${size.width}mm; } }
    </style></head><body>${labels}<script>
      window.onload = function () { setTimeout(function () { window.print(); }, 150); };
      window.onafterprint = function () { window.close(); };
    <\/script></body></html>`);
  printWindow.document.close();
}
