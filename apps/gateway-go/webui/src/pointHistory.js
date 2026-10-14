import { reactive } from 'vue';
const HISTORY_LIMIT = 120, SERIES_LIMIT = 256;
const histories = reactive(new Map());
const touched = new Map();
export function pointStatusKey(status) { return `${status.deviceKey}::${status.metric}`; }
function snapshotValue(value) {
    if (!value || typeof value !== 'object')
        return value;
    try {
        return JSON.parse(JSON.stringify(value));
    }
    catch {
        return String(value);
    }
}
function pruneSeries() {
    if (histories.size < SERIES_LIMIT)
        return;
    let oldestKey = '', oldest = Number.POSITIVE_INFINITY;
    touched.forEach((value, key) => { if (value < oldest) {
        oldest = value;
        oldestKey = key;
    } });
    if (oldestKey) {
        histories.delete(oldestKey);
        touched.delete(oldestKey);
    }
}
export function recordPointStatuses(points) {
    const now = Date.now();
    for (const status of points) {
        if (!status?.deviceKey || !status?.metric || status.value === undefined || status.value === null)
            continue;
        const key = pointStatusKey(status), at = status.receiveTimestamp || status.updatedAt || new Date(now).toISOString();
        const existing = histories.get(key) || [];
        const previous = existing[existing.length - 1];
        if (previous?.at === at)
            continue;
        if (!histories.has(key))
            pruneSeries();
        const next = [...existing, { at, value: snapshotValue(status.value), quality: status.quality, sourceTimestamp: status.sourceTimestamp }].slice(-HISTORY_LIMIT);
        histories.set(key, next);
        touched.set(key, now);
    }
}
export function pointHistory(key) { return histories.get(key) || []; }
