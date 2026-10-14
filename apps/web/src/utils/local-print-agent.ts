import { createGatewayQrDataUrl, gatewayLabelSizes, type GatewayLabelData } from './gateway-label-print';

const defaultUrl = 'http://127.0.0.1:17896';

export interface LocalPrintAgentSettings {
  enabled: boolean;
  url: string;
  pairingKey: string;
  printer: string;
}

export function getLocalPrintAgentSettings(): LocalPrintAgentSettings {
  try {
    const saved = JSON.parse(localStorage.getItem('gateway-local-print-agent') || '{}');
    return {
      enabled: Boolean(saved.enabled),
      url: String(saved.url || defaultUrl).replace(/\/$/, ''),
      pairingKey: String(saved.pairingKey || ''),
      printer: String(saved.printer || ''),
    };
  } catch {
    return { enabled: false, url: defaultUrl, pairingKey: '', printer: '' };
  }
}

export function saveLocalPrintAgentSettings(settings: LocalPrintAgentSettings) {
  localStorage.setItem('gateway-local-print-agent', JSON.stringify({
    ...settings,
    url: settings.url.replace(/\/$/, ''),
  }));
}

async function agentRequest<T>(path: string, settings: LocalPrintAgentSettings, init: RequestInit = {}) {
  const response = await fetch(`${settings.url}${path}`, {
    ...init,
    cache: 'no-store',
    headers: {
      'Content-Type': 'application/json',
      ...(settings.pairingKey ? { 'X-Weikong-Print-Key': settings.pairingKey } : {}),
      ...(init.headers || {}),
    },
    signal: AbortSignal.timeout(3500),
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.message || `打印服务响应异常（${response.status}）`);
  return data as T;
}

export function testLocalPrintAgent(settings: LocalPrintAgentSettings) {
  return agentRequest<{ ok: boolean; version: string; defaultPrinter: string }>('/api/health', settings);
}

export function loadLocalPrinters(settings: LocalPrintAgentSettings) {
  return agentRequest<{ ok: boolean; printers: string[]; defaultPrinter: string }>('/api/printers', settings);
}

export async function printWithLocalAgent(
  items: GatewayLabelData[],
  sizeValue: string,
  copies: number,
  settings = getLocalPrintAgentSettings(),
) {
  if (!settings.enabled || !settings.pairingKey) return false;
  const size = gatewayLabelSizes.find((item) => item.value === sizeValue) || gatewayLabelSizes[0];
  const renderedItems = await Promise.all(items.map(async (item) => ({
    sn: item.sn,
    bindCode: item.bindCode,
    qrDataUrl: await createGatewayQrDataUrl(item, 480),
  })));
  await agentRequest('/api/print', settings, {
    method: 'POST',
    body: JSON.stringify({
      printer: settings.printer || undefined,
      widthMm: size.width,
      heightMm: size.height,
      copies,
      items: renderedItems,
    }),
  });
  return true;
}
