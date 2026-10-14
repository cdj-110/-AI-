/** Keep legacy nicknames visible while presenting one user-facing name everywhere. */
export function userDisplayName(user?: { username?: string | null; nickname?: string | null } | null) {
  return user?.nickname?.trim() || user?.username?.trim() || '—';
}
