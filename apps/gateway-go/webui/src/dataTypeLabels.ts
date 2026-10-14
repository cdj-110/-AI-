// Presentation only: stored configuration and API type codes stay unchanged.
const labels: Record<string, string> = {
  auto: '自动识别', bool: '布尔值', boolean: '布尔值',
  float32: '32位浮点数', float64: '64位浮点数',
  string: '字符串', datetime: '日期时间', timestamp: '时间戳',
  quality: '质量码', object: '结构化数据', currency: '货币数值',
}
export function dataTypeLabel(value: unknown): string {
  const original = String(value ?? '').trim()
  if (!original || original === '-' || original === '—') return '—'
  const code = original.toLowerCase()
  if (code.endsWith('[]')) return dataTypeLabel(code.slice(0, -2)) + '数组'
  const integer = /^(u?)int(8|16|32|64)$/.exec(code)
  if (integer) return integer[2] + '位' + (integer[1] ? '无符号' : '有符号') + '整数'
  return labels[code] || original
}
