export type PointMetricSource = { metric?: unknown }

export function nextGeneratedPointMetric(points: PointMetricSource[], prefix: string): string {
  const normalizedPrefix = String(prefix || 'point').trim() || 'point'
  const used = new Set(points.map(point => String(point.metric || '').trim().toLowerCase()))
  const pattern = new RegExp(`^${normalizedPrefix}-P(\\d+)$`, 'i')
  let next = 1
  for (const point of points) {
    const match = String(point.metric || '').trim().match(pattern)
    if (match) next = Math.max(next, Number(match[1]) + 1)
  }
  let metric = ''
  do metric = `${normalizedPrefix}-P${String(next++).padStart(2, '0')}`
  while (used.has(metric.toLowerCase()))
  return metric
}

export function generatedPointName(namePrefix: string, metric: string): string {
  return `${String(namePrefix || 'device').trim() || 'device'}@${String(metric || '').trim()}`
}
