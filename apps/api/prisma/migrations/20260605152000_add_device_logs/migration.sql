CREATE TABLE "DeviceLog" (
  "id" TEXT NOT NULL,
  "tenantId" TEXT,
  "deviceId" TEXT,
  "deviceKey" TEXT NOT NULL,
  "deviceName" TEXT,
  "type" TEXT NOT NULL,
  "level" TEXT NOT NULL DEFAULT 'INFO',
  "source" TEXT NOT NULL,
  "message" TEXT NOT NULL,
  "detail" JSONB,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

  CONSTRAINT "DeviceLog_pkey" PRIMARY KEY ("id")
);

CREATE INDEX "DeviceLog_tenantId_idx" ON "DeviceLog"("tenantId");
CREATE INDEX "DeviceLog_deviceId_idx" ON "DeviceLog"("deviceId");
CREATE INDEX "DeviceLog_deviceKey_idx" ON "DeviceLog"("deviceKey");
CREATE INDEX "DeviceLog_type_idx" ON "DeviceLog"("type");
CREATE INDEX "DeviceLog_level_idx" ON "DeviceLog"("level");
CREATE INDEX "DeviceLog_source_idx" ON "DeviceLog"("source");
CREATE INDEX "DeviceLog_createdAt_idx" ON "DeviceLog"("createdAt");

ALTER TABLE "DeviceLog" ADD CONSTRAINT "DeviceLog_tenantId_fkey" FOREIGN KEY ("tenantId") REFERENCES "Tenant"("id") ON DELETE SET NULL ON UPDATE CASCADE;
ALTER TABLE "DeviceLog" ADD CONSTRAINT "DeviceLog_deviceId_fkey" FOREIGN KEY ("deviceId") REFERENCES "Device"("id") ON DELETE SET NULL ON UPDATE CASCADE;
