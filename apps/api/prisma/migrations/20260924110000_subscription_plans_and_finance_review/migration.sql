CREATE TABLE "SubscriptionPlan" (
  "id" TEXT NOT NULL, "code" VARCHAR(40) NOT NULL, "name" VARCHAR(60) NOT NULL, "description" VARCHAR(300),
  "deviceLimit" INTEGER NOT NULL DEFAULT 0, "pointLimit" INTEGER NOT NULL DEFAULT 0,
  "scadaProjectLimit" INTEGER NOT NULL DEFAULT 0, "userLimit" INTEGER NOT NULL DEFAULT 0,
  "monthlyPriceCents" INTEGER NOT NULL DEFAULT 0, "priceCents" INTEGER NOT NULL DEFAULT 0,
  "currency" VARCHAR(8) NOT NULL DEFAULT 'CNY', "enabled" BOOLEAN NOT NULL DEFAULT true,
  "sortOrder" INTEGER NOT NULL DEFAULT 0, "gracePeriodDays" INTEGER NOT NULL DEFAULT 7,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP, "updatedAt" TIMESTAMP(3) NOT NULL,
  CONSTRAINT "SubscriptionPlan_pkey" PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX "SubscriptionPlan_code_key" ON "SubscriptionPlan"("code");
CREATE INDEX "SubscriptionPlan_enabled_sortOrder_idx" ON "SubscriptionPlan"("enabled", "sortOrder");

ALTER TABLE "Tenant" ADD COLUMN "subscriptionPlanId" TEXT;
ALTER TABLE "Tenant" ADD COLUMN "subscriptionStartedAt" TIMESTAMP(3);
CREATE INDEX "Tenant_subscriptionPlanId_idx" ON "Tenant"("subscriptionPlanId");
ALTER TABLE "Tenant" ADD CONSTRAINT "Tenant_subscriptionPlanId_fkey" FOREIGN KEY ("subscriptionPlanId") REFERENCES "SubscriptionPlan"("id") ON DELETE SET NULL ON UPDATE CASCADE;

CREATE TABLE "TenantSubscription" (
  "id" TEXT NOT NULL, "tenantId" TEXT NOT NULL, "planId" TEXT NOT NULL, "status" TEXT NOT NULL DEFAULT 'ACTIVE',
  "billingCycle" TEXT NOT NULL DEFAULT 'YEARLY', "startedAt" TIMESTAMP(3) NOT NULL, "expiresAt" TIMESTAMP(3) NOT NULL,
  "graceEndsAt" TIMESTAMP(3) NOT NULL, "baseDeviceLimit" INTEGER NOT NULL, "basePointLimit" INTEGER NOT NULL,
  "baseScadaProjectLimit" INTEGER NOT NULL, "baseUserLimit" INTEGER NOT NULL,
  "addonDeviceLimit" INTEGER NOT NULL DEFAULT 0, "addonPointLimit" INTEGER NOT NULL DEFAULT 0,
  "addonScadaProjectLimit" INTEGER NOT NULL DEFAULT 0, "addonUserLimit" INTEGER NOT NULL DEFAULT 0,
  "autoRenew" BOOLEAN NOT NULL DEFAULT false, "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt" TIMESTAMP(3) NOT NULL, CONSTRAINT "TenantSubscription_pkey" PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX "TenantSubscription_tenantId_key" ON "TenantSubscription"("tenantId");
CREATE INDEX "TenantSubscription_status_expiresAt_idx" ON "TenantSubscription"("status", "expiresAt");
CREATE INDEX "TenantSubscription_graceEndsAt_idx" ON "TenantSubscription"("graceEndsAt");
ALTER TABLE "TenantSubscription" ADD CONSTRAINT "TenantSubscription_tenantId_fkey" FOREIGN KEY ("tenantId") REFERENCES "Tenant"("id") ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE "TenantSubscription" ADD CONSTRAINT "TenantSubscription_planId_fkey" FOREIGN KEY ("planId") REFERENCES "SubscriptionPlan"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

CREATE TABLE "QuotaAddon" (
  "id" TEXT NOT NULL, "code" VARCHAR(40) NOT NULL, "name" VARCHAR(80) NOT NULL, "description" VARCHAR(300),
  "resourceType" TEXT NOT NULL, "quantity" INTEGER NOT NULL, "priceCents" INTEGER NOT NULL,
  "currency" VARCHAR(8) NOT NULL DEFAULT 'CNY', "enabled" BOOLEAN NOT NULL DEFAULT true, "sortOrder" INTEGER NOT NULL DEFAULT 0,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP, "updatedAt" TIMESTAMP(3) NOT NULL,
  CONSTRAINT "QuotaAddon_pkey" PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX "QuotaAddon_code_key" ON "QuotaAddon"("code");
CREATE INDEX "QuotaAddon_enabled_sortOrder_idx" ON "QuotaAddon"("enabled", "sortOrder");

CREATE TABLE "SubscriptionOrder" (
  "id" TEXT NOT NULL, "orderNo" VARCHAR(32) NOT NULL, "tenantId" TEXT NOT NULL, "subscriptionId" TEXT,
  "orderType" TEXT NOT NULL, "targetPlanId" TEXT, "addonId" TEXT, "addonQuantity" INTEGER NOT NULL DEFAULT 1,
  "billingCycle" TEXT NOT NULL DEFAULT 'YEARLY', "durationMonths" INTEGER NOT NULL DEFAULT 12,
  "amountCents" INTEGER NOT NULL, "currency" VARCHAR(8) NOT NULL DEFAULT 'CNY', "status" TEXT NOT NULL DEFAULT 'PENDING_REVIEW',
  "requestedById" TEXT NOT NULL, "requestedByName" TEXT NOT NULL, "requestedAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "activatedAt" TIMESTAMP(3), "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP, "updatedAt" TIMESTAMP(3) NOT NULL,
  CONSTRAINT "SubscriptionOrder_pkey" PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX "SubscriptionOrder_orderNo_key" ON "SubscriptionOrder"("orderNo");
CREATE INDEX "SubscriptionOrder_tenantId_requestedAt_idx" ON "SubscriptionOrder"("tenantId", "requestedAt");
CREATE INDEX "SubscriptionOrder_status_requestedAt_idx" ON "SubscriptionOrder"("status", "requestedAt");
CREATE INDEX "SubscriptionOrder_targetPlanId_idx" ON "SubscriptionOrder"("targetPlanId");
ALTER TABLE "SubscriptionOrder" ADD CONSTRAINT "SubscriptionOrder_tenantId_fkey" FOREIGN KEY ("tenantId") REFERENCES "Tenant"("id") ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE "SubscriptionOrder" ADD CONSTRAINT "SubscriptionOrder_subscriptionId_fkey" FOREIGN KEY ("subscriptionId") REFERENCES "TenantSubscription"("id") ON DELETE SET NULL ON UPDATE CASCADE;
ALTER TABLE "SubscriptionOrder" ADD CONSTRAINT "SubscriptionOrder_targetPlanId_fkey" FOREIGN KEY ("targetPlanId") REFERENCES "SubscriptionPlan"("id") ON DELETE RESTRICT ON UPDATE CASCADE;
ALTER TABLE "SubscriptionOrder" ADD CONSTRAINT "SubscriptionOrder_addonId_fkey" FOREIGN KEY ("addonId") REFERENCES "QuotaAddon"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

CREATE TABLE "PaymentProof" (
  "id" TEXT NOT NULL, "orderId" TEXT NOT NULL, "voucherFileName" TEXT NOT NULL, "voucherOriginalName" TEXT NOT NULL,
  "voucherMimeType" TEXT NOT NULL, "paymentReference" VARCHAR(100), "paidBy" VARCHAR(100), "paidAt" TIMESTAMP(3),
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP, CONSTRAINT "PaymentProof_pkey" PRIMARY KEY ("id")
);
CREATE INDEX "PaymentProof_orderId_idx" ON "PaymentProof"("orderId");
ALTER TABLE "PaymentProof" ADD CONSTRAINT "PaymentProof_orderId_fkey" FOREIGN KEY ("orderId") REFERENCES "SubscriptionOrder"("id") ON DELETE CASCADE ON UPDATE CASCADE;

CREATE TABLE "FinanceReview" (
  "id" TEXT NOT NULL, "orderId" TEXT NOT NULL, "decision" TEXT NOT NULL, "reviewerId" TEXT NOT NULL,
  "reviewerName" TEXT NOT NULL, "note" VARCHAR(500), "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT "FinanceReview_pkey" PRIMARY KEY ("id")
);
CREATE INDEX "FinanceReview_orderId_createdAt_idx" ON "FinanceReview"("orderId", "createdAt");
ALTER TABLE "FinanceReview" ADD CONSTRAINT "FinanceReview_orderId_fkey" FOREIGN KEY ("orderId") REFERENCES "SubscriptionOrder"("id") ON DELETE CASCADE ON UPDATE CASCADE;

CREATE TABLE "EntitlementChange" (
  "id" TEXT NOT NULL, "tenantId" TEXT NOT NULL, "subscriptionId" TEXT NOT NULL, "orderId" TEXT, "changeType" TEXT NOT NULL,
  "deviceDelta" INTEGER NOT NULL DEFAULT 0, "pointDelta" INTEGER NOT NULL DEFAULT 0,
  "scadaProjectDelta" INTEGER NOT NULL DEFAULT 0, "userDelta" INTEGER NOT NULL DEFAULT 0,
  "effectiveDeviceLimit" INTEGER NOT NULL, "effectivePointLimit" INTEGER NOT NULL,
  "effectiveScadaLimit" INTEGER NOT NULL, "effectiveUserLimit" INTEGER NOT NULL,
  "description" VARCHAR(300), "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT "EntitlementChange_pkey" PRIMARY KEY ("id")
);
CREATE INDEX "EntitlementChange_tenantId_createdAt_idx" ON "EntitlementChange"("tenantId", "createdAt");
ALTER TABLE "EntitlementChange" ADD CONSTRAINT "EntitlementChange_tenantId_fkey" FOREIGN KEY ("tenantId") REFERENCES "Tenant"("id") ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE "EntitlementChange" ADD CONSTRAINT "EntitlementChange_subscriptionId_fkey" FOREIGN KEY ("subscriptionId") REFERENCES "TenantSubscription"("id") ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE "EntitlementChange" ADD CONSTRAINT "EntitlementChange_orderId_fkey" FOREIGN KEY ("orderId") REFERENCES "SubscriptionOrder"("id") ON DELETE SET NULL ON UPDATE CASCADE;

CREATE TABLE "SubscriptionNotification" (
  "id" TEXT NOT NULL, "tenantId" TEXT NOT NULL, "dedupeKey" TEXT NOT NULL, "type" TEXT NOT NULL,
  "title" TEXT NOT NULL, "message" TEXT NOT NULL, "readAt" TIMESTAMP(3), "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT "SubscriptionNotification_pkey" PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX "SubscriptionNotification_dedupeKey_key" ON "SubscriptionNotification"("dedupeKey");
CREATE INDEX "SubscriptionNotification_tenantId_createdAt_idx" ON "SubscriptionNotification"("tenantId", "createdAt");
CREATE INDEX "SubscriptionNotification_tenantId_readAt_idx" ON "SubscriptionNotification"("tenantId", "readAt");
ALTER TABLE "SubscriptionNotification" ADD CONSTRAINT "SubscriptionNotification_tenantId_fkey" FOREIGN KEY ("tenantId") REFERENCES "Tenant"("id") ON DELETE CASCADE ON UPDATE CASCADE;

INSERT INTO "SubscriptionPlan" (
  "id", "code", "name", "description", "deviceLimit", "pointLimit", "scadaProjectLimit", "userLimit",
  "monthlyPriceCents", "priceCents", "currency", "enabled", "sortOrder", "gracePeriodDays", "updatedAt"
) VALUES (
  '00000000-0000-4000-8000-000000000001', 'BASIC', '基础版', '现有租户默认版本，可在版本管理中调整新购价格和额度。',
  20, 2000, 5, 10, 0, 0, 'CNY', true, 10, 7, CURRENT_TIMESTAMP
);

INSERT INTO "QuotaAddon" ("id", "code", "name", "resourceType", "quantity", "priceCents", "currency", "enabled", "sortOrder", "updatedAt") VALUES
  ('00000000-0000-4000-8100-000000000001', 'DEVICE_10', '设备扩容包（10台）', 'DEVICE', 10, 10000, 'CNY', true, 10, CURRENT_TIMESTAMP),
  ('00000000-0000-4000-8100-000000000002', 'POINT_1000', '点位扩容包（1000点）', 'POINT', 1000, 10000, 'CNY', true, 20, CURRENT_TIMESTAMP),
  ('00000000-0000-4000-8100-000000000003', 'SCADA_5', '组态扩容包（5个）', 'SCADA', 5, 10000, 'CNY', true, 30, CURRENT_TIMESTAMP),
  ('00000000-0000-4000-8100-000000000004', 'USER_5', '用户扩容包（5人）', 'USER', 5, 10000, 'CNY', true, 40, CURRENT_TIMESTAMP);

UPDATE "Tenant" SET "subscriptionPlanId" = '00000000-0000-4000-8000-000000000001', "subscriptionStartedAt" = CURRENT_TIMESTAMP WHERE "subscriptionPlanId" IS NULL;

INSERT INTO "TenantSubscription" (
  "id", "tenantId", "planId", "status", "billingCycle", "startedAt", "expiresAt", "graceEndsAt",
  "baseDeviceLimit", "basePointLimit", "baseScadaProjectLimit", "baseUserLimit", "updatedAt"
)
SELECT gen_random_uuid()::text, "id", '00000000-0000-4000-8000-000000000001', 'ACTIVE', 'YEARLY', CURRENT_TIMESTAMP,
       CURRENT_TIMESTAMP + INTERVAL '1 year', CURRENT_TIMESTAMP + INTERVAL '1 year 7 days', 20, 2000, 5, 10, CURRENT_TIMESTAMP
FROM "Tenant";
