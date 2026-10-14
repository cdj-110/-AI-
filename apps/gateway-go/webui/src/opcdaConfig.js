export function normalizeOPCDAReadMode(value) {
    return value === 'subscription' ? 'subscription' : 'sync';
}
