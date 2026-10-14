ALTER TABLE "Device"
ADD COLUMN "cloudAccessEnabled" BOOLEAN NOT NULL DEFAULT true,
ADD COLUMN "cloudDisabledAt" TIMESTAMP(3),
ADD COLUMN "cloudDisabledBy" TEXT,
ADD COLUMN "cloudDisabledReason" TEXT;

CREATE INDEX "Device_cloudAccessEnabled_idx" ON "Device"("cloudAccessEnabled");
