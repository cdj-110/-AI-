CREATE TABLE "GatewayMaintenanceTask" (
    "id" TEXT NOT NULL,
    "gatewayId" TEXT NOT NULL,
    "action" TEXT NOT NULL,
    "status" TEXT NOT NULL DEFAULT 'PENDING',
    "message" TEXT,
    "createdBy" TEXT NOT NULL,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "expiresAt" TIMESTAMP(3) NOT NULL,
    "completedAt" TIMESTAMP(3),
    CONSTRAINT "GatewayMaintenanceTask_pkey" PRIMARY KEY ("id")
);

CREATE INDEX "GatewayMaintenanceTask_gatewayId_createdAt_idx" ON "GatewayMaintenanceTask"("gatewayId", "createdAt");
CREATE INDEX "GatewayMaintenanceTask_status_expiresAt_idx" ON "GatewayMaintenanceTask"("status", "expiresAt");
ALTER TABLE "GatewayMaintenanceTask" ADD CONSTRAINT "GatewayMaintenanceTask_gatewayId_fkey" FOREIGN KEY ("gatewayId") REFERENCES "FactoryGateway"("id") ON DELETE RESTRICT ON UPDATE CASCADE;
