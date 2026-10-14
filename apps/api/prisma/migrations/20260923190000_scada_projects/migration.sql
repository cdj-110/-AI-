CREATE TABLE "ScadaProject" (
    "id" TEXT NOT NULL,
    "tenantId" TEXT NOT NULL,
    "name" VARCHAR(80) NOT NULL,
    "description" VARCHAR(300),
    "status" TEXT NOT NULL DEFAULT 'DRAFT',
    "thumbnail" TEXT,
    "config" JSONB NOT NULL,
    "createdBy" TEXT NOT NULL,
    "updatedBy" TEXT NOT NULL,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "ScadaProject_pkey" PRIMARY KEY ("id")
);

CREATE INDEX "ScadaProject_tenantId_updatedAt_idx" ON "ScadaProject"("tenantId", "updatedAt");
CREATE INDEX "ScadaProject_tenantId_status_idx" ON "ScadaProject"("tenantId", "status");

ALTER TABLE "ScadaProject"
ADD CONSTRAINT "ScadaProject_tenantId_fkey"
FOREIGN KEY ("tenantId") REFERENCES "Tenant"("id") ON DELETE CASCADE ON UPDATE CASCADE;
