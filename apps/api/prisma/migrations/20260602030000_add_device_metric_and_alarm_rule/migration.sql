CREATE TABLE "DeviceMetric" (
    "id" TEXT NOT NULL,
    "deviceId" TEXT NOT NULL,
    "identifier" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "dataType" TEXT NOT NULL,
    "unit" TEXT,
    "decimals" INTEGER NOT NULL DEFAULT 2,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "DeviceMetric_pkey" PRIMARY KEY ("id")
);

CREATE TABLE "DeviceAlarmRule" (
    "id" TEXT NOT NULL,
    "deviceId" TEXT NOT NULL,
    "identifier" TEXT NOT NULL,
    "operator" TEXT NOT NULL,
    "threshold" DOUBLE PRECISION NOT NULL,
    "level" TEXT NOT NULL DEFAULT 'WARNING',
    "enabled" BOOLEAN NOT NULL DEFAULT true,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "DeviceAlarmRule_pkey" PRIMARY KEY ("id")
);

CREATE INDEX "DeviceMetric_deviceId_idx" ON "DeviceMetric"("deviceId");
CREATE UNIQUE INDEX "DeviceMetric_deviceId_identifier_key" ON "DeviceMetric"("deviceId", "identifier");
CREATE INDEX "DeviceAlarmRule_deviceId_idx" ON "DeviceAlarmRule"("deviceId");
CREATE UNIQUE INDEX "DeviceAlarmRule_deviceId_identifier_operator_key" ON "DeviceAlarmRule"("deviceId", "identifier", "operator");

ALTER TABLE "DeviceMetric" ADD CONSTRAINT "DeviceMetric_deviceId_fkey" FOREIGN KEY ("deviceId") REFERENCES "Device"("id") ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE "DeviceAlarmRule" ADD CONSTRAINT "DeviceAlarmRule_deviceId_fkey" FOREIGN KEY ("deviceId") REFERENCES "Device"("id") ON DELETE CASCADE ON UPDATE CASCADE;
