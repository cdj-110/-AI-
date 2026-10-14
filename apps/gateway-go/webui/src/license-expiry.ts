// Only an explicit, signed empty expiry denotes a perpetual license.
export function formatLicenseExpiry(value?: string) {
  if (value === '') return '永久有效'
  if (!value) return '未记录'
  const date = new Date(value)
  return Number.isFinite(date.getTime()) ? date.toLocaleString('zh-CN', {hour12:false}) : '有效期异常'
}
