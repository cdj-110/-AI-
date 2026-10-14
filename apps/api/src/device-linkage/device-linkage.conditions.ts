export function linkageMatches(value: unknown, operator: string, threshold: number): boolean | undefined {
  if (typeof value !== 'number' || !Number.isFinite(value) || !Number.isFinite(threshold)) return undefined;
  switch (operator) {
    case 'GT': return value > threshold;
    case 'GTE': return value >= threshold;
    case 'LT': return value < threshold;
    case 'LTE': return value <= threshold;
    case 'EQ': return value === threshold;
    case 'NE': return value !== threshold;
    default: return undefined;
  }
}
export function freshLinkageSample(time: string, enabledAt: Date | null, lastAt: Date | null, now = Date.now()) {
  const sample = new Date(time).getTime();
  return Number.isFinite(sample) && sample <= now && now - sample <= 30_000
    && !!enabledAt && sample > enabledAt.getTime() && (!lastAt || sample > lastAt.getTime());
}

export function hasLinkageCycle(rules: Array<{ sourceDeviceId: string; sourceMetric: string; targetDeviceId: string; targetMetric: string }>) {
  const edges = new Map<string, string[]>();
  for (const rule of rules) {
    const from = JSON.stringify([rule.sourceDeviceId, rule.sourceMetric]), to = JSON.stringify([rule.targetDeviceId, rule.targetMetric]);
    edges.set(from, [...(edges.get(from) || []), to]);
  }
  const visited = new Set<string>(), visiting = new Set<string>();
  const visit = (node: string): boolean => {
    if (visiting.has(node)) return true;
    if (visited.has(node)) return false;
    visiting.add(node);
    if ((edges.get(node) || []).some(visit)) return true;
    visiting.delete(node); visited.add(node); return false;
  };
  return [...edges.keys()].some(visit);
}
