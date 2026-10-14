import { Injectable, Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { Pool } from 'pg';
import { PrismaService } from '../prisma/prisma.service';
import { SubscriptionLimitsService } from './subscription-limits.service';

@Injectable()
export class HistoryCleanupService implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(HistoryCleanupService.name);
  private readonly pool = new Pool({ connectionString: process.env.TIMESCALE_DATABASE_URL ?? 'postgresql://weikong@localhost:5433/weikong_ts' });
  private timer?: ReturnType<typeof setInterval>;
  private busy = false;
  constructor(private readonly prisma: PrismaService, private readonly limits: SubscriptionLimitsService) {}
  onModuleInit() {
    // Default is off: enabling requires a reviewed impact report and explicit activation date.
    if (process.env.HISTORY_CLEANUP_ENABLED !== 'true') return;
    this.timer = setInterval(() => void this.run().catch(e => this.logger.error(e.message)), 3_600_000);
    this.timer.unref?.();
  }
  async onModuleDestroy() { if (this.timer) clearInterval(this.timer); await this.pool.end(); }
  async impact(tenantId: string, now = new Date()) {
    const retention = await this.limits.history(tenantId, now);
    const devices = await this.prisma.device.findMany({ where: { tenantId }, select: { id: true, deviceKey: true } });
    // Only globally unique device ids are safe for deletion. Legacy keys/orphans need a separate audit.
    const ids = devices.map(d => d.id);
    const result = await this.pool.query('SELECT COUNT(*)::text AS count, MIN(time) AS "oldestAt", MAX(time) AS "newestAt" FROM telemetry_events WHERE device_id = ANY($1::text[]) AND time < $2', [ids, retention.cleanupCutoffAt]);
    return { tenantId, retention, devices: devices.length, affectedRows: Number(result.rows[0]?.count ?? 0), oldestAt: result.rows[0]?.oldestAt, newestAt: result.rows[0]?.newestAt, cleanupEnabled: process.env.HISTORY_CLEANUP_ENABLED === 'true', ids };
  }
  async run(now = new Date()) {
    const activated = new Date(process.env.HISTORY_CLEANUP_ACTIVATED_AT ?? 'invalid');
    if (this.busy || process.env.HISTORY_CLEANUP_ENABLED !== 'true' || !Number.isFinite(activated.getTime()) || now.getTime() < activated.getTime() + 7 * 86_400_000) return;
    this.busy = true;
    try {
      let cursor: string | undefined;
      while (true) {
        const tenants = await this.prisma.tenant.findMany({ orderBy: { id: 'asc' }, take: 100, ...(cursor ? { cursor: { id: cursor }, skip: 1 } : {}), select: { id: true } });
        if (!tenants.length) break;
        for (const tenant of tenants) {
          // Re-read entitlement for each bounded batch. Never use a global drop_chunks policy.
          for (let batch = 0; batch < 5; batch++) {
            const count = await this.prisma.$transaction(async tx => {
              await tx.$queryRaw`SELECT "id" FROM "Tenant" WHERE "id" = ${tenant.id} FOR UPDATE`;
              await tx.$queryRaw`SELECT "id" FROM "TenantSubscription" WHERE "tenantId" = ${tenant.id} FOR UPDATE`;
              await tx.$queryRaw`SELECT p."id" FROM "SubscriptionPlan" p JOIN "Tenant" t ON t."subscriptionPlanId" = p."id" WHERE t."id" = ${tenant.id} FOR UPDATE OF p`;
              const report = await this.impact(tenant.id, new Date());
              if (!report.ids.length || !report.affectedRows) return 0;
              const deleted = await this.pool.query(`WITH doomed AS (
              SELECT tableoid, ctid FROM telemetry_events WHERE device_id = ANY($1::text[]) AND time < $2 ORDER BY time LIMIT 1000
            ) DELETE FROM telemetry_events e USING doomed d WHERE e.tableoid = d.tableoid AND e.ctid = d.ctid`, [report.ids, report.retention.cleanupCutoffAt]);
              return deleted.rowCount ?? 0;
            }, { timeout: 30_000 });
            this.logger.log(`history cleanup tenant=${tenant.id} rows=${count}`);
            if (count < 1000) break;
          }
        }
        cursor = tenants[tenants.length - 1].id;
      }
    } finally { this.busy = false; }
  }
}
