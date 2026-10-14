export interface GatewayIdentity {
  hardwareId: string;
  chipUsid: string;
  gatewayKey?: string;
}

/** Parse the full text copied by the gateway's pending-activation page. */
export function parseGatewayIdentity(text: string): GatewayIdentity {
  const fields: Record<string, string> = {};
  for (const line of text.replace(/^\uFEFF/, '').split(/\r?\n/)) {
    const match = line.trim().match(/^(网关(?:编号)?|gatewayKey|HardwareId|(?:RJGT102\s*)?芯片\s*USID|chipUsid)\s*[:：]\s*(.*?)\s*$/i);
    if (!match) continue;
    const label = match[1].toLowerCase();
    const key = label === 'hardwareid' ? 'hardwareId'
      : label.includes('usid') ? 'chipUsid' : 'gatewayKey';
    const value = key === 'gatewayKey' ? match[2] : match[2].toUpperCase();
    if (fields[key] !== undefined && fields[key] !== value) {
      throw new Error('设备信息包含多个不同的设备身份，请只粘贴一台网关的信息');
    }
    fields[key] = value;
  }
  if (!fields.hardwareId || !fields.chipUsid) {
    throw new Error('请粘贴网关“复制设备身份”的完整内容，需包含 HardwareId 和芯片 USID');
  }
  if (!/^[0-9A-F]{32}$/.test(fields.hardwareId)) {
    throw new Error('HardwareId 应为完整的 32 位十六进制硬件指纹');
  }
  if (!/^[0-9A-F]{16}$/.test(fields.chipUsid)) {
    throw new Error('芯片 USID 应为完整的 16 位十六进制身份');
  }
  return { hardwareId: fields.hardwareId, chipUsid: fields.chipUsid, gatewayKey: fields.gatewayKey };
}
