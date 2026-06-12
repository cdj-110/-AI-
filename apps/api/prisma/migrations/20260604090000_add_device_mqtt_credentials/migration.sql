ALTER TABLE "Device"
ADD COLUMN "mqttClientId" TEXT,
ADD COLUMN "mqttUsername" TEXT,
ADD COLUMN "mqttPasswordHash" TEXT,
ADD COLUMN "mqttPasswordUpdatedAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP;

UPDATE "Device"
SET
  "mqttClientId" = 'wk_' || regexp_replace("deviceKey", '[^a-zA-Z0-9_-]', '_', 'g') || '_' || substr("id", 1, 8),
  "mqttUsername" = 'device:' || substr("id", 1, 8),
  "mqttPasswordHash" = '$2b$10$fpQWmnPjidOfbhLJ1o1aHekpp8l1NLfdG8W2h1q/x/reH4ercrPPW'
WHERE "mqttClientId" IS NULL;

ALTER TABLE "Device" ALTER COLUMN "mqttClientId" SET NOT NULL;
ALTER TABLE "Device" ALTER COLUMN "mqttUsername" SET NOT NULL;
ALTER TABLE "Device" ALTER COLUMN "mqttPasswordHash" SET NOT NULL;

CREATE UNIQUE INDEX "Device_mqttClientId_key" ON "Device"("mqttClientId");
CREATE UNIQUE INDEX "Device_mqttUsername_key" ON "Device"("mqttUsername");
