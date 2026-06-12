CREATE TABLE "DeviceModelTemplate" (
    "id" TEXT NOT NULL,
    "tenantId" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "description" TEXT,
    "deviceType" TEXT NOT NULL DEFAULT 'DIRECT',
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "DeviceModelTemplate_pkey" PRIMARY KEY ("id")
);

CREATE TABLE "DeviceModelMetric" (
    "id" TEXT NOT NULL,
    "templateId" TEXT NOT NULL,
    "identifier" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "dataType" TEXT NOT NULL,
    "unit" TEXT,
    "decimals" INTEGER NOT NULL DEFAULT 2,
    "accessMode" TEXT NOT NULL DEFAULT 'READ_ONLY',
    "enabled" BOOLEAN NOT NULL DEFAULT true,
    "sortOrder" INTEGER NOT NULL DEFAULT 0,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "DeviceModelMetric_pkey" PRIMARY KEY ("id")
);

CREATE INDEX "DeviceModelTemplate_tenantId_idx" ON "DeviceModelTemplate"("tenantId");
CREATE INDEX "DeviceModelTemplate_deviceType_idx" ON "DeviceModelTemplate"("deviceType");
CREATE INDEX "DeviceModelMetric_templateId_idx" ON "DeviceModelMetric"("templateId");
CREATE UNIQUE INDEX "DeviceModelMetric_templateId_identifier_key" ON "DeviceModelMetric"("templateId", "identifier");

ALTER TABLE "DeviceModelTemplate"
ADD CONSTRAINT "DeviceModelTemplate_tenantId_fkey"
FOREIGN KEY ("tenantId") REFERENCES "Tenant"("id")
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE "DeviceModelMetric"
ADD CONSTRAINT "DeviceModelMetric_templateId_fkey"
FOREIGN KEY ("templateId") REFERENCES "DeviceModelTemplate"("id")
ON DELETE CASCADE ON UPDATE CASCADE;
