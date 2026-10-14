CREATE TABLE "DeviceLinkageRule" (
  "id" TEXT NOT NULL, "tenantId" TEXT NOT NULL, "name" VARCHAR(80) NOT NULL,
  "sourceDeviceId" TEXT NOT NULL, "sourceName" TEXT NOT NULL, "sourceMetric" TEXT NOT NULL,
  "operator" TEXT NOT NULL, "threshold" DOUBLE PRECISION NOT NULL,
  "targetDeviceId" TEXT NOT NULL, "targetName" TEXT NOT NULL, "targetMetric" TEXT NOT NULL,
  "actionValue" JSONB NOT NULL, "enabled" BOOLEAN NOT NULL DEFAULT false,
  "enabledAt" TIMESTAMP(3), "authorizedBy" TEXT NOT NULL, "revision" INTEGER NOT NULL DEFAULT 1,
  "met" BOOLEAN NOT NULL DEFAULT false, "lastSampleAt" TIMESTAMP(3), "lastError" TEXT,
  "deletedAt" TIMESTAMP(3), "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt" TIMESTAMP(3) NOT NULL, CONSTRAINT "DeviceLinkageRule_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "DeviceLinkageRule_tenantId_fkey" FOREIGN KEY ("tenantId") REFERENCES "Tenant"("id") ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE TABLE "DeviceLinkageRun" (
  "id" TEXT NOT NULL, "ruleId" TEXT NOT NULL, "requestId" TEXT NOT NULL,
  "targetDeviceId" TEXT NOT NULL, "targetMetric" TEXT NOT NULL,
  "observedValue" DOUBLE PRECISION NOT NULL, "actionValue" JSONB NOT NULL,
  "sampleAt" TIMESTAMP(3) NOT NULL, "status" TEXT NOT NULL DEFAULT 'DISPATCHING',
  "message" TEXT, "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT "DeviceLinkageRun_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "DeviceLinkageRun_ruleId_fkey" FOREIGN KEY ("ruleId") REFERENCES "DeviceLinkageRule"("id") ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE INDEX "DeviceLinkageRule_tenantId_deletedAt_createdAt_idx" ON "DeviceLinkageRule"("tenantId", "deletedAt", "createdAt");
CREATE INDEX "DeviceLinkageRule_sourceDeviceId_enabled_idx" ON "DeviceLinkageRule"("sourceDeviceId", "enabled");
CREATE UNIQUE INDEX "DeviceLinkageRun_requestId_key" ON "DeviceLinkageRun"("requestId");
CREATE INDEX "DeviceLinkageRun_ruleId_createdAt_idx" ON "DeviceLinkageRun"("ruleId", "createdAt");
CREATE INDEX "DeviceLinkageRun_targetDeviceId_targetMetric_status_idx" ON "DeviceLinkageRun"("targetDeviceId", "targetMetric", "status");
