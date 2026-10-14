import { ref } from 'vue'
import { request } from '@/utils/request'
import { readPlatformToken } from '@/utils/platformAuth'
import { formatHistoryDate } from './platformHistoryChart'

export const historyWindow = ref<{ queryEarliestAt: string; latestAt: string; label: string; graceEndsAt?: string } | null>(null)
let pending: Promise<void> | null = null, loadedAt = 0, loadedToken = ''
export async function refreshHistoryWindow() {
  const token = readPlatformToken()
  if (token !== loadedToken) { historyWindow.value = null; loadedAt = 0; loadedToken = token }
  if (historyWindow.value && Date.now() - loadedAt < 60_000) return
  if (!pending) pending = request({ url: '/conf/history-retention', method: 'GET' }).then((data: any) => {
    if (token !== readPlatformToken()) return
    historyWindow.value = data; loadedAt = Date.now()
  }).finally(() => { pending = null })
  await pending
}
export const historyDateDisabled = (timestamp: number) => {
  if (timestamp > Date.now()) return true
  if (!historyWindow.value) return true
  // Date picker supplies day midnight; keep the cutoff day selectable and validate exact time on query.
  return timestamp + 86400000 <= new Date(historyWindow.value.queryEarliestAt).getTime()
}
export const historyHoursAllowed = (hours: number) => !historyWindow.value || Date.now() - hours * 3600000 >= new Date(historyWindow.value.queryEarliestAt).getTime()
export const historyWindowDescription = () => historyWindow.value
  ? `历史保存：${historyWindow.value.label}；可查询自 ${formatHistoryDate(new Date(historyWindow.value.queryEarliestAt))}${historyWindow.value.graceEndsAt ? `；7天清理宽限至 ${formatHistoryDate(new Date(historyWindow.value.graceEndsAt))}` : ''}`
  : '正在读取历史保存期限'
export function normalizeHistoryRange(range: string[] | null) {
  if (!historyWindow.value) throw new Error('无法确认历史保存期限，请重试')
  const end = new Date()
  const cutoff = new Date(historyWindow.value.queryEarliestAt).getTime()
  const parse = (v: string) => new Date(`${v.replace(' ', 'T')}+08:00`).getTime()
  if (range?.length === 2 && parse(range[0]) >= cutoff && parse(range[1]) <= end.getTime() && parse(range[0]) < parse(range[1])) return { range, adjusted: false }
  return { range: [formatHistoryDate(new Date(Math.max(cutoff + 1000, end.getTime() - 3600000))), formatHistoryDate(end)], adjusted: true }
}
