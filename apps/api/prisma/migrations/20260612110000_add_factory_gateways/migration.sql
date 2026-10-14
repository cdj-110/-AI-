CREATE TABLE "FactoryGateway" (
  "id" TEXT NOT NULL,
  "hardwareId" TEXT NOT NULL,
  "sn" TEXT NOT NULL,
  "deviceSecretHash" TEXT NOT NULL,
  "bindCodeHash" TEXT NOT NULL,
  "status" TEXT NOT NULL DEFAULT 'UNBOUND',
  "batchNo" TEXT,
  "deviceId" TEXT,
  "producedAt" TIMESTAMP(3),
  "firstSeenAt" TIMESTAMP(3),
  "lastSeenAt" TIMESTAMP(3),
  "boundAt" TIMESTAMP(3),
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt" TIMESTAMP(3) NOT NULL,
  CONSTRAINT "FactoryGateway_pkey" PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX "FactoryGateway_hardwareId_key" ON "FactoryGateway"("hardwareId");
CREATE UNIQUE INDEX "FactoryGateway_sn_key" ON "FactoryGateway"("sn");
CREATE UNIQUE INDEX "FactoryGateway_deviceId_key" ON "FactoryGateway"("deviceId");
CREATE INDEX "FactoryGateway_status_idx" ON "FactoryGateway"("status");
CREATE INDEX "FactoryGateway_batchNo_idx" ON "FactoryGateway"("batchNo");
CREATE INDEX "FactoryGateway_lastSeenAt_idx" ON "FactoryGateway"("lastSeenAt");

ALTER TABLE "FactoryGateway"
ADD CONSTRAINT "FactoryGateway_deviceId_fkey"
FOREIGN KEY ("deviceId") REFERENCES "Device"("id") ON DELETE SET NULL ON UPDATE CASCADE;
