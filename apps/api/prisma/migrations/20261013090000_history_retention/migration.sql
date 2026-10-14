-- Additive candidate only. No telemetry is removed by this migration.
ALTER TABLE "SubscriptionPlan" ADD COLUMN "historyRetentionDays" INTEGER NOT NULL DEFAULT 7,
 ADD COLUMN "historyRetentionMonths" INTEGER NOT NULL DEFAULT 0,
 ADD COLUMN "historyGraceCutoffAt" TIMESTAMP(3), ADD COLUMN "historyGraceEndsAt" TIMESTAMP(3);
UPDATE "SubscriptionPlan" SET "historyRetentionDays" = 0,
 "historyRetentionMonths" = CASE "code" WHEN 'BASIC' THEN 6 WHEN 'ENTERPRISE' THEN 9 WHEN 'PROFESSIONAL' THEN 12 END
 WHERE "code" IN ('BASIC', 'ENTERPRISE', 'PROFESSIONAL');
ALTER TABLE "SubscriptionPlan" ADD CONSTRAINT "history_retention_unit" CHECK
 (("historyRetentionDays" > 0 AND "historyRetentionMonths" = 0) OR ("historyRetentionDays" = 0 AND "historyRetentionMonths" > 0));
ALTER TABLE "TenantSubscription" ADD COLUMN "addonHistoryMonths" INTEGER NOT NULL DEFAULT 0,
 ADD COLUMN "historyAddonExpiresAt" TIMESTAMP(3), ADD COLUMN "historyGraceCutoffAt" TIMESTAMP(3),
 ADD COLUMN "historyGraceEndsAt" TIMESTAMP(3);
ALTER TABLE "SubscriptionOrder" ADD COLUMN "historyAddonMonths" INTEGER NOT NULL DEFAULT 0;
ALTER TABLE "QuotaAddon" ADD COLUMN "pricePending" BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE "EntitlementChange" ADD COLUMN "historyMonthsDelta" INTEGER NOT NULL DEFAULT 0;
INSERT INTO "QuotaAddon" ("id","code","name","description","resourceType","quantity","priceCents","pricePending","enabled","updatedAt")
 VALUES ('bbfc475c-75af-4fea-89f8-130b86c846ec','HISTORY_3_MONTHS','历史存储扩展包',
 '每份延长3个自然月，随购买时的订阅到期；价格待定，暂不销售。不能恢复已清理的数据。','HISTORY',3,0,true,false,CURRENT_TIMESTAMP)
 ON CONFLICT ("code") DO NOTHING;
