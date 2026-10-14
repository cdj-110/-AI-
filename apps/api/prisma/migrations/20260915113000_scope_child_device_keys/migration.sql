-- Root devices still own their MQTT namespace and therefore keep a globally
-- unique device key. Gateway children are identified inside one gateway, so
-- the same local key may safely exist below different gateways.
DROP INDEX IF EXISTS "Device_deviceKey_key";

CREATE UNIQUE INDEX "Device_root_deviceKey_key"
ON "Device"("deviceKey")
WHERE "gatewayId" IS NULL;

CREATE UNIQUE INDEX "Device_gateway_deviceKey_key"
ON "Device"("gatewayId", "deviceKey")
WHERE "gatewayId" IS NOT NULL;

CREATE INDEX "Device_gatewayId_deviceKey_idx"
ON "Device"("gatewayId", "deviceKey");
