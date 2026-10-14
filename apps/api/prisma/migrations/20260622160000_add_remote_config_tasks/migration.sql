CREATE TABLE "RemoteConfigTask" (
  "id" TEXT NOT NULL,
  "deviceId" TEXT NOT NULL,
  "version" INTEGER NOT NULL,
  "status" TEXT NOT NULL DEFAULT 'PENDING',
  "config" JSONB NOT NULL,
  "createdBy" TEXT NOT NULL,
  "error" TEXT,
  "sentAt" TIMESTAMP(3),
  "completedAt" TIMESTAMP(3),
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updatedAt" TIMESTAMP(3) NOT NULL,
  CONSTRAINT "RemoteConfigTask_pkey" PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX "RemoteConfigTask_deviceId_version_key" ON "RemoteConfigTask"("deviceId", "version");
CREATE INDEX "RemoteConfigTask_deviceId_idx" ON "RemoteConfigTask"("deviceId");
CREATE INDEX "RemoteConfigTask_status_idx" ON "RemoteConfigTask"("status");
CREATE INDEX "RemoteConfigTask_createdAt_idx" ON "RemoteConfigTask"("createdAt");
ALTER TABLE "RemoteConfigTask" ADD CONSTRAINT "RemoteConfigTask_deviceId_fkey" FOREIGN KEY ("deviceId") REFERENCES "Device"("id") ON DELETE CASCADE ON UPDATE CASCADE;
