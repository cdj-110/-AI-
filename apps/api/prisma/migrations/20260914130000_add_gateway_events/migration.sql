CREATE TABLE "GatewayEvent" (
    "id" TEXT NOT NULL,
    "dedupeKey" TEXT NOT NULL,
    "tenantId" TEXT NOT NULL,
    "gatewayId" TEXT NOT NULL,
    "deviceId" TEXT,
    "sourceEventId" TEXT NOT NULL,
    "eventType" TEXT NOT NULL DEFAULT 'ALARM',
    "state" TEXT NOT NULL,
    "level" TEXT NOT NULL,
    "ruleId" TEXT,
    "ruleName" TEXT,
    "parentRuleId" TEXT,
    "parentRuleName" TEXT,
    "limitKey" TEXT,
    "limitName" TEXT,
    "deviceKey" TEXT,
    "pointName" TEXT,
    "metric" TEXT,
    "valueText" TEXT,
    "unit" TEXT,
    "threshold" DOUBLE PRECISION,
    "quality" TEXT,
    "message" TEXT NOT NULL,
    "acknowledged" BOOLEAN NOT NULL DEFAULT false,
    "occurredAt" TIMESTAMP(3) NOT NULL,
    "sourceUpdatedAt" TIMESTAMP(3) NOT NULL,
    "acknowledgedAt" TIMESTAMP(3),
    "recoveredAt" TIMESTAMP(3),
    "reportedAt" TIMESTAMP(3) NOT NULL,
    "detail" JSONB,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,
    CONSTRAINT "GatewayEvent_pkey" PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX "GatewayEvent_dedupeKey_key" ON "GatewayEvent"("dedupeKey");
CREATE INDEX "GatewayEvent_tenantId_occurredAt_idx" ON "GatewayEvent"("tenantId", "occurredAt");
CREATE INDEX "GatewayEvent_gatewayId_occurredAt_idx" ON "GatewayEvent"("gatewayId", "occurredAt");
CREATE INDEX "GatewayEvent_deviceId_occurredAt_idx" ON "GatewayEvent"("deviceId", "occurredAt");
CREATE INDEX "GatewayEvent_state_occurredAt_idx" ON "GatewayEvent"("state", "occurredAt");
CREATE INDEX "GatewayEvent_level_occurredAt_idx" ON "GatewayEvent"("level", "occurredAt");
CREATE INDEX "GatewayEvent_sourceEventId_idx" ON "GatewayEvent"("sourceEventId");
ALTER TABLE "GatewayEvent" ADD CONSTRAINT "GatewayEvent_tenantId_fkey" FOREIGN KEY ("tenantId") REFERENCES "Tenant"("id") ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE "GatewayEvent" ADD CONSTRAINT "GatewayEvent_gatewayId_fkey" FOREIGN KEY ("gatewayId") REFERENCES "Device"("id") ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE "GatewayEvent" ADD CONSTRAINT "GatewayEvent_deviceId_fkey" FOREIGN KEY ("deviceId") REFERENCES "Device"("id") ON DELETE SET NULL ON UPDATE CASCADE;
