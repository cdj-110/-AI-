// Natural calendar months in Asia/Shanghai; clamp the day for February/month end.
export const HISTORY_GRACE_DAYS = 7;
const DAY = 86_400_000;
export function shiftHistoryMonths(date: Date, months: number): Date {
  const local = new Date(date.getTime() + 8 * 3_600_000);
  const day = local.getUTCDate();
  local.setUTCDate(1);
  local.setUTCMonth(local.getUTCMonth() + months);
  const last = new Date(Date.UTC(local.getUTCFullYear(), local.getUTCMonth() + 1, 0)).getUTCDate();
  local.setUTCDate(Math.min(day, last));
  return new Date(local.getTime() - 8 * 3_600_000);
}
type Plan = { historyRetentionDays?: number; historyRetentionMonths?: number; historyGraceCutoffAt?: Date | null; historyGraceEndsAt?: Date | null };
type Subscription = { expiresAt: Date; addonHistoryMonths?: number; historyAddonExpiresAt?: Date | null; historyGraceCutoffAt?: Date | null; historyGraceEndsAt?: Date | null; status?: string };
export function historyRetention(plan: Plan | null | undefined, subscription?: Subscription | null, now = new Date()) {
  const expired = subscription && subscription.expiresAt <= now;
  const months = expired ? 0 : (plan?.historyRetentionMonths ?? 0);
  const days = expired ? 7 : (months ? 0 : (plan?.historyRetentionDays || 7));
  const addonExpiresAt = subscription?.historyAddonExpiresAt
    ? new Date(Math.min(subscription.historyAddonExpiresAt.getTime(), subscription.expiresAt.getTime())) : null;
  const addonMonths = addonExpiresAt && addonExpiresAt > now ? subscription?.addonHistoryMonths ?? 0 : 0;
  const earliestAt = new Date(shiftHistoryMonths(now, -(months + addonMonths)).getTime() - days * DAY);
  const graceActive = Boolean(subscription?.historyGraceEndsAt && subscription.historyGraceEndsAt > now && subscription.historyGraceCutoffAt);
  let queryEarliestAt = graceActive && subscription!.historyGraceCutoffAt! < earliestAt ? subscription!.historyGraceCutoffAt! : earliestAt;
  let graceEndsAt = graceActive ? subscription!.historyGraceEndsAt! : null;
  // Plan-level protection also covers tenants assigned a plan without a paid subscription row.
  if (plan?.historyGraceEndsAt && plan.historyGraceEndsAt > now && plan.historyGraceCutoffAt && plan.historyGraceCutoffAt < queryEarliestAt) {
    queryEarliestAt = plan.historyGraceCutoffAt; graceEndsAt = plan.historyGraceEndsAt;
  }
  // Compute expiry protection even before the hourly notification worker has run.
  for (const lossAt of [subscription?.expiresAt, addonExpiresAt]) {
    if (!lossAt || lossAt > now || lossAt.getTime() + HISTORY_GRACE_DAYS * DAY <= now.getTime()) continue;
    const baseMonths = plan?.historyRetentionMonths ?? 0;
    const baseDays = baseMonths ? 0 : (plan?.historyRetentionDays || 7);
    const priorAddon = addonExpiresAt && addonExpiresAt >= lossAt ? subscription?.addonHistoryMonths ?? 0 : 0;
    const oldCutoff = new Date(shiftHistoryMonths(lossAt, -(baseMonths + priorAddon)).getTime() - baseDays * DAY);
    if (oldCutoff < queryEarliestAt) { queryEarliestAt = oldCutoff; graceEndsAt = new Date(lossAt.getTime() + HISTORY_GRACE_DAYS * DAY); }
  }
  const cleanupCutoffAt = new Date(Math.min(earliestAt.getTime() - HISTORY_GRACE_DAYS * DAY, queryEarliestAt.getTime()));
  return { days, months, addonMonths, addonExpiresAt, earliestAt, queryEarliestAt, cleanupCutoffAt,
    latestAt: now, graceEndsAt, cleanupGraceDays: HISTORY_GRACE_DAYS,
    maxQueryDays: 31, maxPoints: 20, maxPageSize: 100, chartMaxRows: 5000,
    label: `${months ? `${months} 个自然月` : `${days} 天`}${addonMonths ? ` + 扩展 ${addonMonths} 个月` : ''}` };
}
export function protectHistoryReduction(before: ReturnType<typeof historyRetention>, after: ReturnType<typeof historyRetention>, at: Date) {
  if (after.earliestAt <= before.queryEarliestAt) return {};
  return { historyGraceCutoffAt: before.queryEarliestAt, historyGraceEndsAt: new Date(at.getTime() + HISTORY_GRACE_DAYS * DAY) };
}
