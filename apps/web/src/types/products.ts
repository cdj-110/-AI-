export interface ProductModel {
  code: string;
  name: string;
  ethernetPortCount: number | null;
  serialPortCount: number | null;
  rs232PortCount: number | null;
  rs485PortCount: number | null;
  canPortCount: number | null;
  supports4G: boolean | null;
  supportsWifi: boolean | null;
  enabled: boolean;
  description: string | null;
  _count?: { gateways: number };
}

export function supportText(value: boolean | null) {
  return value === true ? '支持' : value === false ? '不支持' : '待确认';
}

export function productSpecification(product: ProductModel) {
  return `网口 ${product.ethernetPortCount ?? '待确认'} · RS232 ${product.rs232PortCount ?? '待确认'} · RS485 ${product.rs485PortCount ?? '待确认'} · CAN ${product.canPortCount ?? '待确认'} · 4G ${supportText(product.supports4G)} · WiFi ${supportText(product.supportsWifi)}`;
}
