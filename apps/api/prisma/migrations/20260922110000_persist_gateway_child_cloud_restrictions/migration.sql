CREATE TABLE "GatewayChildCloudRestriction" (
  "id" TEXT NOT NULL,
  "gatewayDeviceId" TEXT NOT NULL,
  "childDeviceKey" TEXT NOT NULL,
  "disabledAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "disabledBy" TEXT,
  "reason" TEXT,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

  CONSTRAINT "GatewayChildCloudRestriction_pkey" PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX "GatewayChildCloudRestriction_gatewayDeviceId_childDeviceKey_key"
ON "GatewayChildCloudRestriction"("gatewayDeviceId", "childDeviceKey");

CREATE INDEX "GatewayChildCloudRestriction_childDeviceKey_idx"
ON "GatewayChildCloudRestriction"("childDeviceKey");

CREATE INDEX "GatewayChildCloudRestriction_disabledAt_idx"
ON "GatewayChildCloudRestriction"("disabledAt");

ALTER TABLE "GatewayChildCloudRestriction"
ADD CONSTRAINT "GatewayChildCloudRestriction_gatewayDeviceId_fkey"
FOREIGN KEY ("gatewayDeviceId") REFERENCES "Device"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- Backfill restrictions for child devices that were disabled before this table existed.
-- The record is keyed by the parent gateway and child deviceKey, so deleting the child
-- device row cannot silently remove the platform-level restriction.
INSERT INTO "GatewayChildCloudRestriction" (
  "id", "gatewayDeviceId", "childDeviceKey", "disabledAt", "disabledBy", "reason", "createdAt", "updatedAt"
)
SELECT
  md5("gatewayId" || ':' || "deviceKey")::uuid::text,
  "gatewayId",
  "deviceKey",
  COALESCE("cloudDisabledAt", CURRENT_TIMESTAMP),
  "cloudDisabledBy",
  COALESCE("cloudDisabledReason", '管理平台禁用'),
  CURRENT_TIMESTAMP,
  CURRENT_TIMESTAMP
FROM "Device"
WHERE "deviceType" = 'GATEWAY_CHILD'
  AND "gatewayId" IS NOT NULL
  AND "cloudAccessEnabled" = false
ON CONFLICT ("gatewayDeviceId", "childDeviceKey") DO NOTHING;
