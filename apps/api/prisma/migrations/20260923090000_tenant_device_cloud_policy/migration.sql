-- Tenant-wide restrictions are independent of each device's own restriction.
ALTER TABLE "Tenant" ADD COLUMN "deviceCloudAccessDisabled" BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE "Tenant" ADD CONSTRAINT "Tenant_cloud_restriction_requires_disabled"
  CHECK (NOT "deviceCloudAccessDisabled" OR "status" = 'DISABLED');
