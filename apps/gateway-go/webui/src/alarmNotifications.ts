export type AlarmLevel = 'INFO' | 'WARNING' | 'CRITICAL'

export interface AlarmEvent {
  eventId: string
  ruleId: string
  ruleName: string
  parentRuleId?: string
  parentRuleName?: string
  limitKey?: string
  limitName?: string
  threshold?: number
  unit?: string
  deviceKey: string
  metric: string
  pointName?: string
  level: AlarmLevel | string
  state: string
  acknowledged: boolean
  popupEnabled?: boolean
  soundWhenPopupDisabled?: boolean
  retainAfterAcknowledged?: boolean
  value?: unknown
  valueText?: string
  quality?: string
  message: string
  occurredAt: string
  updatedAt: string
  recoveredAt?: string
}

export interface AlarmSnapshot {
	popupDurationSeconds?: number
  active: AlarmEvent[]
  history: AlarmEvent[]
  activeCount: number
  criticalCount: number
  unackedCount: number
}

export const emptyAlarmSnapshot = (): AlarmSnapshot => ({
  active: [],
  history: [],
  activeCount: 0,
  criticalCount: 0,
  unackedCount: 0,
})

export function detectNewActiveAlarms(
  previousEventIds: ReadonlySet<string>,
  snapshot: AlarmSnapshot,
  initialized: boolean,
): { eventIds: Set<string>; events: AlarmEvent[] } {
  const eventIds = new Set(snapshot.active.map(event => event.eventId))
  const events = initialized
    ? snapshot.active.filter(event => !event.acknowledged && !previousEventIds.has(event.eventId))
    : []
  return { eventIds, events }
}

export function alarmPopupEnabled(event: AlarmEvent): boolean {
  return event.popupEnabled !== false
}

export function alarmPanelEligible(event: AlarmEvent): boolean {
  return alarmPopupEnabled(event)&&(!event.acknowledged||event.retainAfterAcknowledged===true)
}

export function alarmSoundEligible(event: AlarmEvent): boolean {
  return alarmPopupEnabled(event)||event.soundWhenPopupDisabled===true
}

export interface AlarmNotification {
  groupId: string
  ruleName: string
  event: AlarmEvent
  events: AlarmEvent[]
  revision: number
}

export function alarmNotificationKey(event: AlarmEvent): string {
  // Use explicit parent metadata, never infer ancestry from names or ID suffixes.
  return JSON.stringify([event.parentRuleId||event.ruleId,event.deviceKey,event.metric])
}

const severityRank:Record<string,number>={CRITICAL:3,WARNING:2,INFO:1}
const limitRank:Record<string,number>={highHigh:4,lowLow:3,high:2,low:1}
export function updateAlarmNotifications(previous:readonly AlarmNotification[],active:readonly AlarmEvent[],fresh:readonly AlarmEvent[]):AlarmNotification[] {
  const groups=new Map<string,AlarmEvent[]>()
  for(const event of active){
    if(!alarmPanelEligible(event))continue
    const key=alarmNotificationKey(event),items=groups.get(key)||[]
    items.push(event);groups.set(key,items)
  }
  const freshKeys=new Set(fresh.filter(alarmPanelEligible).map(alarmNotificationKey))
  const make=(key:string,revision:number):AlarmNotification=>{
    const events=sortActiveAlarms(groups.get(key)!).sort((a,b)=>(severityRank[b.level]||0)-(severityRank[a.level]||0)||(limitRank[b.limitKey||'']||0)-(limitRank[a.limitKey||'']||0))
    const event=events[0]
    return {groupId:key,ruleName:event.parentRuleName||event.ruleName,event,events,revision}
  }
  // Preserve mounted group IDs and positions. Polls/recovery do not restart timers;
  // newly triggered limits reset that card's timer so escalation can be noticed.
  const retained=previous.filter(n=>groups.has(n.groupId)).map(n=>make(n.groupId,n.revision+(freshKeys.has(n.groupId)?1:0)))
  const existing=new Set(retained.map(n=>n.groupId))
  const added=[...freshKeys].filter(key=>groups.has(key)&&!existing.has(key)).map(key=>make(key,1))
  // The unified panel scrolls internally; never silently discard the fourth rule.
  return [...added,...retained]
}

// Panel contents follow the active snapshot, never a dismissal timer. Preserve
// mounted groups/order while refreshing values and adding any missing groups.
export function updatePersistentAlarmNotifications(previous:readonly AlarmNotification[],active:readonly AlarmEvent[]):AlarmNotification[] {
  const eligible=active.filter(event=>event.state==='ACTIVE'&&alarmPanelEligible(event))
  const existing=new Set(previous.map(notification=>notification.groupId))
  const missing=sortActiveAlarms(eligible).filter(event=>!existing.has(alarmNotificationKey(event)))
  return updateAlarmNotifications(previous,eligible,missing)
}

export function alarmLimitText(event:AlarmEvent):string {
  const name=event.limitName||event.ruleName
  const operator=event.limitKey==='high'||event.limitKey==='highHigh'?'>':event.limitKey==='low'||event.limitKey==='lowLow'?'<':''
  return operator&&event.threshold!==undefined?`${name} ${operator} ${event.threshold}${event.unit?' '+event.unit:''}`:name
}

// Copy before sorting; live values, acknowledgements and update times must not
// move existing rows. IDs break ties even when the API returns an unordered list.
export function sortActiveAlarms<T extends Pick<AlarmEvent,'occurredAt'|'ruleId'|'eventId'>>(events: readonly T[]): T[] {
  const compare=(a:string,b:string)=>a<b?-1:a>b?1:0
  return [...events].sort((a,b)=>{
    const timeA=Date.parse(a.occurredAt)||0,timeB=Date.parse(b.occurredAt)||0
    return timeB-timeA||compare(a.ruleId,b.ruleId)||compare(a.eventId,b.eventId)
  })
}

export function alarmValueText(event: AlarmEvent): string {
  if (event.valueText) return event.valueText
  if (event.value === null || event.value === undefined) return '-'
  if (typeof event.value === 'string') return event.value
  try { return JSON.stringify(event.value) }
  catch { return String(event.value) }
}
