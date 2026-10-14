import { componentKeyOf } from './platformBindingCapabilities'

const chartKeys = new Set([
  'BarCommon', 'BarCrossrange', 'BarLine', 'LineCommon', 'LineGradients',
  'LineGradientSingle', 'LineLinearSingle', 'VChartBarCommon', 'VChartBarCrossrange',
  'VChartBarStack', 'VChartLine'
])
export const supportsHistoryChartQuery = (component: any) => chartKeys.has(componentKeyOf(component))
export const isHistoryQueryChart = (component: any) =>
  supportsHistoryChartQuery(component) && component?.request?.requestHttpType === '历史数据'
export const formatHistoryDate = (date: Date) => {
  date = new Date(date.getTime() + 8 * 3600000)
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())} ${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())}:${pad(date.getUTCSeconds())}`
}
export function validateHistoryRange(range: string[] | null): string {
  if (!range || range.length !== 2) return '请选择开始和结束时间'
  const times = range.map(value => new Date(`${value.replace(' ', 'T')}+08:00`).getTime())
  if (times.some(time => !Number.isFinite(time)) || times[0] >= times[1]) return '结束时间必须晚于开始时间'
  if (times[1] > Date.now() + 1000) return '不能查询未来时间'
  return times[1] - times[0] > 31 * 86400000 ? '单次历史查询不能超过31天' : ''
}
// 去除设备名称列，点位分别作为数值系列，按时间升序展示。
export function historyChartDataset(dimensions: any[], rows: any[], option: any, vchart = false): any {
  const columns = dimensions.map(column => typeof column === 'string' ? { key: column, title: column } : column)
    .filter(column => column?.key && !['timestamp', 'deviceName', 'key'].includes(column.key))
  const timeline = new Map<string, any>()
  for (const row of rows) {
    if (!row.timestamp) continue
    const target = timeline.get(row.timestamp) || { timestamp: row.timestamp }
    columns.forEach(column => {
      const value = row[column.key]
      if (value !== null && value !== undefined && value !== '' && Number.isFinite(Number(value))) target[column.key] = Number(value)
    })
    timeline.set(row.timestamp, target)
  }
  const source = [...timeline.values()].sort((a, b) => a.timestamp.localeCompare(b.timestamp))
  if (vchart) {
    const horizontal = option.direction === 'horizontal'
    option.xField = horizontal ? 'historyValue' : 'historyTime'
    option.yField = horizontal ? 'historyTime' : 'historyValue'
    option.seriesField = 'historySeries'
    return source.flatMap(row => columns.filter(column => row[column.key] !== undefined).map(column => ({
      historyTime: row.timestamp, historyValue: row[column.key], historySeries: column.title || column.key
    })))
  }
  return { dimensions: ['timestamp', ...columns.map(column => ({ name: column.key, displayName: column.title || column.key }))], source }
}
export async function fetchHistoryChart(query: (params: any) => Promise<any>, params: any, stale: () => boolean, maxRows = 5000) {
  const limit = Math.min(5000, Math.max(100, Math.floor(maxRows / 100) * 100))
  const rows: any[] = []
  let dimensions: any[] = [], total = 0
  for (let page = 1; page <= limit / 100; page++) {
    if (stale()) return null
    const result = await query({ ...params, chart: true, page, pageSize: 100 })
    if (stale()) return null
    if (!Array.isArray(result?.dataset?.source)) throw new Error('历史查询未返回有效数据')
    dimensions = result.dataset.dimensions || []
    rows.push(...result.dataset.source)
    total = Number(result.total) || rows.length
    if (!result.dataset.source.length || rows.length >= total || result.hasNext === false) break
  }
  return { dimensions, rows, total, limited: total > rows.length }
}
