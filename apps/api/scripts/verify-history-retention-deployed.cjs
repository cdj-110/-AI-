// Read-only deployed-container verification. No session creation, charges, or telemetry writes.
const assert = require('node:assert/strict');
const { PrismaClient } = require('@prisma/client');
const { Pool } = require('pg');
const { DevicesService } = require('./dist/devices/devices.service');
const { SubscriptionLimitsService } = require('./dist/subscriptions/subscription-limits.service');
const { HistoryCleanupService } = require('./dist/subscriptions/history-cleanup.service');
async function main() {
  assert.equal(process.env.HISTORY_CLEANUP_ENABLED, 'false');
  const prisma = new PrismaClient(), pool = new Pool({ connectionString: process.env.TIMESCALE_DATABASE_URL });
  const limits = new SubscriptionLimitsService(prisma), cleanup = new HistoryCleanupService(prisma, limits);
  try {
    const plans = await prisma.subscriptionPlan.findMany({ select: { code: true, name: true, historyRetentionDays: true, historyRetentionMonths: true } });
    for (const [code, days, months] of [['TRIAL',7,0],['BASIC',0,6],['ENTERPRISE',0,9],['PROFESSIONAL',0,12]]) {
      const plan = plans.find(p => p.code === code); assert.ok(plan); assert.equal(plan.historyRetentionDays,days); assert.equal(plan.historyRetentionMonths,months);
    }
    assert.equal(plans.find(p=>p.code==='TRIAL').name,'体验版'); assert.equal(plans.find(p=>p.code==='BASIC').name,'基础版');
    const addon = await prisma.quotaAddon.findUnique({ where: { code: 'HISTORY_3_MONTHS' } });
    assert.ok(addon && addon.pricePending && !addon.enabled && addon.quantity === 3);
    const user = await prisma.user.findFirst({ where: { role: 'TENANT_ADMIN', status: 'ACTIVE', tenantId: { not: null } }, select: { id: true, role: true, tenantId: true } });
    if (!user) { console.log('PASS deployed tiers, draft addon and cleanup-off; no active tenant admin available for history sample.'); return; }
    const actor = { sub: user.id, role: user.role, tenantId: user.tenantId };
    const summary = await limits.usage(user.tenantId);
    assert.equal(summary.historyRetention.cleanupGraceDays,7);
    const impact = await cleanup.impact(user.tenantId);
    assert.equal(impact.cleanupEnabled,false);
    const metric = await prisma.deviceMetric.findFirst({ where: { enabled: true, ignored: false, device: { tenantId: user.tenantId } }, select: { deviceId: true, identifier: true } });
    if (!metric) { console.log('PASS deployed tiers, draft addon, entitlement and read-only cleanup impact; no queryable point.'); return; }
    const service = Object.create(DevicesService.prototype);
    service.prisma=prisma; service.timescale=pool; service.subscriptionLimits=limits;
    const pairs=[{value1:metric.deviceId,value2:metric.identifier}], end=new Date().toISOString(), start=new Date(Date.now()-300000).toISOString();
    const raw=await service.telemetryHistoryTable(actor,pairs,start,end,1,10);
    const chart=await service.telemetryHistoryTable(actor,pairs,start,end,1,100,undefined,true);
    const exported=await service.exportHistory(actor,pairs,start,end);
    assert.ok(raw.retention && chart.aggregation && typeof exported.csv==='string');
    await assert.rejects(service.telemetryHistoryTable(actor,pairs,'2020-01-01 00:00:00','2020-01-02 00:00:00'),/保存期限/);
    console.log(JSON.stringify({result:'PASS deployed read-only history checks',rawRows:raw.dataset.source.length,chartRows:chart.dataset.source.length,exportRows:exported.exported,cleanupGraceDays:7,cleanupEnabled:false}));
  } finally { await cleanup.onModuleDestroy(); await pool.end(); await prisma.$disconnect(); }
}
main().catch(e=>{console.error(e.message);process.exitCode=1});
