CREATE TABLE "GatewayTopologyDraft" (
  "id" TEXT NOT NULL,
  "gatewayDeviceId" TEXT NOT NULL,
  "gatewayKey" TEXT NOT NULL,
  "reportedConfigVersion" TEXT,
  "items" JSONB NOT NULL,
  "status" TEXT NOT NULL DEFAULT 'PENDING',
  "conflicts" JSONB,
  "reportedAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "confirmedAt" TIMESTAMP(3),
  "confirmedBy" TEXT,
  "ignoredAt" TIMESTAMP(3),
  "ignoredBy" TEXT,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt" TIMESTAMP(3) NOT NULL,
  CONSTRAINT "GatewayTopologyDraft_pkey" PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX "GatewayTopologyDraft_gatewayDeviceId_key" ON "GatewayTopologyDraft"("gatewayDeviceId");
CREATE INDEX "GatewayTopologyDraft_gatewayKey_idx" ON "GatewayTopologyDraft"("gatewayKey");
CREATE INDEX "GatewayTopologyDraft_status_idx" ON "GatewayTopologyDraft"("status");
CREATE INDEX "GatewayTopologyDraft_reportedAt_idx" ON "GatewayTopologyDraft"("reportedAt");

ALTER TABLE "GatewayTopologyDraft"
ADD CONSTRAINT "GatewayTopologyDraft_gatewayDeviceId_fkey"
FOREIGN KEY ("gatewayDeviceId") REFERENCES "Device"("id")
ON DELETE CASCADE ON UPDATE CASCADE;
