-- Only persists query definitions; no telemetry copied/deleted.
CREATE TABLE "HistoryDataTask" (
 "id" TEXT NOT NULL, "tenantId" TEXT NOT NULL, "name" VARCHAR(80) NOT NULL,
 "createdById" TEXT NOT NULL, "createdByName" VARCHAR(100) NOT NULL,
 "pairs" JSONB NOT NULL, "deviceNames" TEXT[] NOT NULL,
 "startAt" TIMESTAMP(3) NOT NULL, "endAt" TIMESTAMP(3) NOT NULL,
 "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP, "updatedAt" TIMESTAMP(3) NOT NULL,
 CONSTRAINT "HistoryDataTask_pkey" PRIMARY KEY ("id"),
 CONSTRAINT "HistoryDataTask_tenantId_fkey" FOREIGN KEY ("tenantId") REFERENCES "Tenant"("id") ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE INDEX "HistoryDataTask_tenantId_createdAt_idx" ON "HistoryDataTask"("tenantId","createdAt");
CREATE INDEX "HistoryDataTask_tenantId_createdById_idx" ON "HistoryDataTask"("tenantId","createdById");
