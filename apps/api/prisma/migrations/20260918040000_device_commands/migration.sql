CREATE TABLE "DeviceCommand" (
  "id" TEXT NOT NULL, "requestId" TEXT NOT NULL,
  "deviceId" TEXT NOT NULL, "gatewayId" TEXT NOT NULL,
  "deviceKey" TEXT NOT NULL, "metric" TEXT NOT NULL, "value" JSONB NOT NULL,
  "status" TEXT NOT NULL DEFAULT 'SENT', "message" TEXT,
  "createdBy" TEXT NOT NULL, "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "expiresAt" TIMESTAMP(3) NOT NULL, "completedAt" TIMESTAMP(3),
  CONSTRAINT "DeviceCommand_pkey" PRIMARY KEY ("id"),
  CONSTRAINT "DeviceCommand_deviceId_fkey" FOREIGN KEY ("deviceId") REFERENCES "Device"("id") ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT "DeviceCommand_gatewayId_fkey" FOREIGN KEY ("gatewayId") REFERENCES "Device"("id") ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE UNIQUE INDEX "DeviceCommand_requestId_key" ON "DeviceCommand"("requestId");
CREATE INDEX "DeviceCommand_deviceId_createdAt_idx" ON "DeviceCommand"("deviceId", "createdAt");
CREATE INDEX "DeviceCommand_status_expiresAt_idx" ON "DeviceCommand"("status", "expiresAt");
