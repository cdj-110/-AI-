CREATE TABLE "Device" (
    "id" TEXT NOT NULL,
    "tenantId" TEXT NOT NULL,
    "deviceKey" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "protocol" TEXT NOT NULL DEFAULT 'MQTT',
    "status" TEXT NOT NULL DEFAULT 'OFFLINE',
    "location" TEXT,
    "lastSeenAt" TIMESTAMP(3),
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,
    CONSTRAINT "Device_pkey" PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX "Device_deviceKey_key" ON "Device"("deviceKey");
CREATE INDEX "Device_tenantId_idx" ON "Device"("tenantId");
CREATE INDEX "Device_status_idx" ON "Device"("status");

ALTER TABLE "Device"
ADD CONSTRAINT "Device_tenantId_fkey"
FOREIGN KEY ("tenantId") REFERENCES "Tenant"("id")
ON DELETE CASCADE ON UPDATE CASCADE;
