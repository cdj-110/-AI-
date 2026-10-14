// Actual types come from the collector, not the configurable wire encoding.
export function dlt645ActualType(value?: {actualDataType?: string}) {
  return value?.actualDataType?.trim() || '—'
}

export function dlt645Version(value?: {diagnostic?: string}, configured = 'auto') {
  const detected = /^DL\/T 645-(2007|1997)$/.exec(value?.diagnostic || '')
  return detected?.[1] || (configured === 'auto' ? '待识别' : configured)
}

export function dlt645Format(format?: string, length?: number) {
  const label = ({
    bcd: 'BCD', 'signed-bcd': '带符号 BCD', binary: '二进制整数',
    ascii: 'ASCII', date: '日期', time: '时间', datetime: '日期时间',
    'demand-time': '需量及发生时间', 'raw-hex': '原始十六进制',
  } as Record<string, string>)[format || ''] || format || '—'
  return Number.isInteger(length) && Number(length) > 0 ? label + ' · ' + length + '字节' : label
}
