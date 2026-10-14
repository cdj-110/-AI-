-- A device can belong to multiple groups. Preserve every existing one-group assignment.
CREATE UNIQUE INDEX "Device_id_tenantId_key" ON "Device"("id", "tenantId");

CREATE TABLE "DeviceGroupMembership" (
  "deviceId" TEXT NOT NULL,
  "groupId" TEXT NOT NULL,
  "tenantId" TEXT NOT NULL,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT "DeviceGroupMembership_pkey" PRIMARY KEY ("deviceId", "groupId")
);

CREATE INDEX "DeviceGroupMembership_tenantId_groupId_idx"
  ON "DeviceGroupMembership"("tenantId", "groupId");

INSERT INTO "DeviceGroupMembership" ("deviceId", "groupId", "tenantId")
SELECT "id", "groupId", "tenantId"
FROM "Device"
WHERE "groupId" IS NOT NULL;

ALTER TABLE "DeviceGroupMembership"
  ADD CONSTRAINT "DeviceGroupMembership_deviceId_tenantId_fkey"
    FOREIGN KEY ("deviceId", "tenantId") REFERENCES "Device"("id", "tenantId")
    ON DELETE CASCADE ON UPDATE CASCADE,
  ADD CONSTRAINT "DeviceGroupMembership_groupId_tenantId_fkey"
    FOREIGN KEY ("groupId", "tenantId") REFERENCES "DeviceGroup"("id", "tenantId")
    ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE "Device" DROP CONSTRAINT "Device_groupId_tenantId_fkey";

-- Keep the legacy column and index for one release as an application rollback aid.
-- New code no longer reads or writes this column; a later cleanup migration can remove it.
