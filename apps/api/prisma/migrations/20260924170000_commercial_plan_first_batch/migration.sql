ALTER TABLE "SubscriptionPlan"
  ADD COLUMN "billingCycles" TEXT[] NOT NULL DEFAULT ARRAY['YEARLY']::TEXT[],
  ADD COLUMN "quoteOnly" BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE "QuotaAddon"
  ADD COLUMN "quoteOnly" BOOLEAN NOT NULL DEFAULT false;

INSERT INTO "SubscriptionPlan" (
  "id", "code", "name", "description", "deviceLimit", "pointLimit", "scadaProjectLimit", "userLimit",
  "monthlyPriceCents", "priceCents", "billingCycles", "quoteOnly", "currency", "enabled", "sortOrder", "gracePeriodDays", "updatedAt"
) VALUES
  ('00000000-0000-4000-8000-000000000002', 'TRIAL', '体验版', '适合产品体验和小规模验证。', 3, 100, 1, 2, 0, 0, ARRAY['YEARLY']::TEXT[], false, 'CNY', true, 10, 7, CURRENT_TIMESTAMP),
  ('00000000-0000-4000-8000-000000000001', 'BASIC', '基础版', '适合小型项目和基础设备接入。', 50, 5000, 5, 10, 0, 298000, ARRAY['YEARLY']::TEXT[], false, 'CNY', true, 20, 7, CURRENT_TIMESTAMP),
  ('00000000-0000-4000-8000-000000000003', 'PROFESSIONAL', '专业版', '适合多设备、多项目的规模化应用。', 200, 30000, 20, 30, 0, 698000, ARRAY['YEARLY']::TEXT[], false, 'CNY', true, 30, 7, CURRENT_TIMESTAMP),
  ('00000000-0000-4000-8000-000000000004', 'ENTERPRISE', '企业版', '适合大规模设备接入和企业级应用。', 500, 100000, 50, 100, 0, 1298000, ARRAY['YEARLY']::TEXT[], false, 'CNY', true, 40, 7, CURRENT_TIMESTAMP)
ON CONFLICT ("code") DO UPDATE SET
  "name" = EXCLUDED."name", "description" = EXCLUDED."description", "deviceLimit" = EXCLUDED."deviceLimit",
  "pointLimit" = EXCLUDED."pointLimit", "scadaProjectLimit" = EXCLUDED."scadaProjectLimit", "userLimit" = EXCLUDED."userLimit",
  "monthlyPriceCents" = EXCLUDED."monthlyPriceCents", "priceCents" = EXCLUDED."priceCents",
  "billingCycles" = EXCLUDED."billingCycles", "quoteOnly" = EXCLUDED."quoteOnly", "currency" = EXCLUDED."currency",
  "enabled" = EXCLUDED."enabled", "sortOrder" = EXCLUDED."sortOrder", "gracePeriodDays" = EXCLUDED."gracePeriodDays",
  "updatedAt" = CURRENT_TIMESTAMP;

-- 旧版演示扩容包保留历史订单关系，但停止继续销售。
UPDATE "QuotaAddon"
SET "enabled" = false, "updatedAt" = CURRENT_TIMESTAMP
WHERE "code" IN ('DEVICE_10', 'POINT_1000', 'SCADA_5', 'USER_5');

INSERT INTO "QuotaAddon" (
  "id", "code", "name", "description", "resourceType", "quantity", "priceCents", "quoteOnly", "currency", "enabled", "sortOrder", "updatedAt"
) VALUES
  ('00000000-0000-4000-8100-000000000010', 'POINT_5000_YEAR', '云点位扩容包（5000点）', '增加 5000 个云上报点位，有效期随当前订阅。', 'POINT', 5000, 50000, false, 'CNY', true, 10, CURRENT_TIMESTAMP),
  ('00000000-0000-4000-8100-000000000011', 'POINT_10000_YEAR', '云点位扩容包（10000点）', '增加 10000 个云上报点位，有效期随当前订阅。', 'POINT', 10000, 80000, false, 'CNY', true, 20, CURRENT_TIMESTAMP),
  ('00000000-0000-4000-8100-000000000012', 'POINT_30000_YEAR', '云点位扩容包（30000点）', '增加 30000 个云上报点位，有效期随当前订阅。', 'POINT', 30000, 200000, false, 'CNY', true, 30, CURRENT_TIMESTAMP),
  ('00000000-0000-4000-8100-000000000013', 'POINT_50000_YEAR', '云点位扩容包（50000点）', '增加 50000 个云上报点位，有效期随当前订阅。', 'POINT', 50000, 300000, false, 'CNY', true, 40, CURRENT_TIMESTAMP),
  ('00000000-0000-4000-8100-000000000014', 'POINT_100000_QUOTE', '云点位扩容包（100000点）', '增加 100000 个云上报点位，需联系平台商务报价。', 'POINT', 100000, 0, true, 'CNY', true, 50, CURRENT_TIMESTAMP)
ON CONFLICT ("code") DO UPDATE SET
  "name" = EXCLUDED."name", "description" = EXCLUDED."description", "resourceType" = EXCLUDED."resourceType",
  "quantity" = EXCLUDED."quantity", "priceCents" = EXCLUDED."priceCents", "quoteOnly" = EXCLUDED."quoteOnly",
  "currency" = EXCLUDED."currency", "enabled" = EXCLUDED."enabled", "sortOrder" = EXCLUDED."sortOrder",
  "updatedAt" = CURRENT_TIMESTAMP;

-- 仅迁移仍保持旧基础版默认快照的租户，人工调整或已扩容的快照不被覆盖。
UPDATE "TenantSubscription"
SET "baseDeviceLimit" = 50, "basePointLimit" = 5000, "baseScadaProjectLimit" = 5, "baseUserLimit" = 10,
    "billingCycle" = 'YEARLY', "updatedAt" = CURRENT_TIMESTAMP
WHERE "planId" = '00000000-0000-4000-8000-000000000001'
  AND "baseDeviceLimit" = 20 AND "basePointLimit" = 2000
  AND "baseScadaProjectLimit" = 5 AND "baseUserLimit" = 10;
