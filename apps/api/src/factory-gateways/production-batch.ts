export function normalizeGatewayModel(value?: string): string {
  const model = value?.trim().toUpperCase() || 'EG100';
  if (!/^[A-Z0-9][A-Z0-9_-]{0,31}$/.test(model)) {
    throw new Error('型号限 32 位字母、数字、横线或下划线，且以字母或数字开头');
  }
  return model;
}

export function productionBatchPrefix(model: string, date = new Date()): string {
  const parts = new Intl.DateTimeFormat('en', {
    timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit',
  }).formatToParts(date);
  const part = (type: string) => parts.find(item => item.type === type)?.value;
  return `${part('year')}${part('month')}${part('day')}-${model}-`;
}

export function highestBatchSequence(prefix: string, batches: { batchNo: string | null }[]): number {
  return batches.reduce((highest, item) => {
    if (!item.batchNo?.startsWith(prefix)) return highest;
    const suffix = item.batchNo.slice(prefix.length);
    if (!/^\d+$/.test(suffix)) return highest;
    const sequence = Number(suffix);
    if (!Number.isSafeInteger(sequence) || sequence > 2147483646) throw new Error('生产批次序号超出支持范围');
    return Math.max(highest, sequence);
  }, 0);
}
