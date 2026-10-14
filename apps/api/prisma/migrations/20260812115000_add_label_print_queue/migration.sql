CREATE TABLE "LabelPrintAgent" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "tokenHash" TEXT NOT NULL,
    "printerName" TEXT,
    "status" TEXT NOT NULL DEFAULT 'OFFLINE',
    "lastSeenAt" TIMESTAMP(3),
    "lastError" TEXT,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,
    CONSTRAINT "LabelPrintAgent_pkey" PRIMARY KEY ("id")
);

CREATE TABLE "LabelPrintJob" (
    "id" TEXT NOT NULL,
    "agentId" TEXT NOT NULL,
    "factoryGatewayId" TEXT NOT NULL,
    "sn" TEXT NOT NULL,
    "labelSize" TEXT NOT NULL DEFAULT '50x30',
    "copies" INTEGER NOT NULL DEFAULT 1,
    "status" TEXT NOT NULL DEFAULT 'PENDING',
    "attempts" INTEGER NOT NULL DEFAULT 0,
    "createdBy" TEXT NOT NULL,
    "claimedAt" TIMESTAMP(3),
    "completedAt" TIMESTAMP(3),
    "error" TEXT,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,
    CONSTRAINT "LabelPrintJob_pkey" PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX "LabelPrintAgent_tokenHash_key" ON "LabelPrintAgent"("tokenHash");
CREATE INDEX "LabelPrintAgent_status_idx" ON "LabelPrintAgent"("status");
CREATE INDEX "LabelPrintAgent_lastSeenAt_idx" ON "LabelPrintAgent"("lastSeenAt");
CREATE INDEX "LabelPrintJob_agentId_status_createdAt_idx" ON "LabelPrintJob"("agentId", "status", "createdAt");
CREATE INDEX "LabelPrintJob_factoryGatewayId_idx" ON "LabelPrintJob"("factoryGatewayId");
CREATE INDEX "LabelPrintJob_createdAt_idx" ON "LabelPrintJob"("createdAt");
ALTER TABLE "LabelPrintJob" ADD CONSTRAINT "LabelPrintJob_agentId_fkey" FOREIGN KEY ("agentId") REFERENCES "LabelPrintAgent"("id") ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE "LabelPrintJob" ADD CONSTRAINT "LabelPrintJob_factoryGatewayId_fkey" FOREIGN KEY ("factoryGatewayId") REFERENCES "FactoryGateway"("id") ON DELETE CASCADE ON UPDATE CASCADE;
