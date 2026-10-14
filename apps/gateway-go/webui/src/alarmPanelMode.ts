export type AlarmPanelMode='page'|'tab'
// Respect an explicit in-page choice, but use a separate tab for new preferences.
export function resolveAlarmPanelMode(value:unknown):AlarmPanelMode{return value==='page'?'page':'tab'}
