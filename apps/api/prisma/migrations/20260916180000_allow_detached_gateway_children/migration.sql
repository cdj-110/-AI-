-- Detached children are retained tenant-owned archives, not root MQTT devices.
-- Their local keys may repeat after multiple gateways have been reclaimed.
DROP INDEX IF EXISTS "Device_root_deviceKey_key";
CREATE UNIQUE INDEX "Device_root_deviceKey_key"
ON "Device"("deviceKey")
WHERE "gatewayId" IS NULL AND "deviceType" <> 'GATEWAY_CHILD';
