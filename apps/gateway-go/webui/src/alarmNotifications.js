export const emptyAlarmSnapshot = () => ({
    active: [],
    history: [],
    activeCount: 0,
    criticalCount: 0,
    unackedCount: 0,
});
export function detectNewActiveAlarms(previousEventIds, snapshot, initialized) {
    const eventIds = new Set(snapshot.active.map(event => event.eventId));
    const events = initialized
        ? snapshot.active.filter(event => !event.acknowledged && !previousEventIds.has(event.eventId))
        : [];
    return { eventIds, events };
}
export function alarmValueText(event) {
    if (event.valueText)
        return event.valueText;
    if (event.value === null || event.value === undefined)
        return '-';
    if (typeof event.value === 'string')
        return event.value;
    try {
        return JSON.stringify(event.value);
    }
    catch {
        return String(event.value);
    }
}
