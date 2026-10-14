export type OPCDAReadMode = 'sync' | 'subscription'

export function normalizeOPCDAReadMode(value: unknown): OPCDAReadMode {
  return value === 'subscription' ? 'subscription' : 'sync'
}
