ALTER TABLE "Device"
ADD COLUMN "deviceType" TEXT NOT NULL DEFAULT 'DIRECT',
ADD COLUMN "gatewayId" TEXT;

CREATE INDEX "Device_deviceType_idx" ON "Device"("deviceType");
CREATE INDEX "Device_gatewayId_idx" ON "Device"("gatewayId");

ALTER TABLE "Device"
ADD CONSTRAINT "Device_gatewayId_fkey"
FOREIGN KEY ("gatewayId") REFERENCES "Device"("id")
ON DELETE SET NULL ON UPDATE CASCADE;
