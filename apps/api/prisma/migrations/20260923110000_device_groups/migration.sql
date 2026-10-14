CREATE TABLE "DeviceGroup" (
  "id" TEXT NOT NULL,
  "tenantId" TEXT NOT NULL,
  "name" VARCHAR(50) NOT NULL,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt" TIMESTAMP(3) NOT NULL,
  CONSTRAINT "DeviceGroup_pkey" PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX "DeviceGroup_id_tenantId_key" ON "DeviceGroup"("id", "tenantId");
CREATE UNIQUE INDEX "DeviceGroup_tenantId_name_key" ON "DeviceGroup"("tenantId", "name");
ALTER TABLE "DeviceGroup" ADD CONSTRAINT "DeviceGroup_tenantId_fkey"
  FOREIGN KEY ("tenantId") REFERENCES "Tenant"("id") ON DELETE CASCADE ON UPDATE CASCADE;
ALTER TABLE "Device" ADD COLUMN "groupId" TEXT;
CREATE INDEX "Device_tenantId_groupId_idx" ON "Device"("tenantId", "groupId");
-- Composite FK prevents assigning a device to another tenant's group, including non-API writes.
-- The service clears membership before removing a group. Devices are never deleted by group removal.
ALTER TABLE "Device" ADD CONSTRAINT "Device_groupId_tenantId_fkey"
  FOREIGN KEY ("groupId", "tenantId") REFERENCES "DeviceGroup"("id", "tenantId") ON DELETE NO ACTION ON UPDATE NO ACTION;
