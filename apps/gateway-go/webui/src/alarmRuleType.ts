export type AlarmRuleType = 'threshold' | 'boolean' | 'quality' | 'stale'

const numericDataType = /^(?:auto|u?int(?:8|16|32|64)?|float(?:32|64)?|single|double|real(?:4|8)?|currency|decimal|number|numeric|vt_(?:i|ui|r)\d)$/i

export function alarmRuleTypeForDataType(dataType?: string): AlarmRuleType {
  const normalized = String(dataType || 'auto').trim().toLowerCase()
  if (normalized === 'bool' || normalized === 'boolean' || normalized === 'vt_bool') return 'boolean'
  if (numericDataType.test(normalized)) return 'threshold'
  return 'quality'
}

export function alarmRuleTypeName(type: AlarmRuleType): string {
  return ({ threshold: '数值阈值', boolean: '布尔状态', quality: '数据质量', stale: '数据陈旧' } as const)[type]
}
