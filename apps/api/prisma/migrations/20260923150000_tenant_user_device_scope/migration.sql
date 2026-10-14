ALTER TABLE "User"
ADD COLUMN "permissions" TEXT[] NOT NULL DEFAULT ARRAY['DASHBOARD_VIEW', 'DEVICE_VIEW', 'MAP_VIEW', 'SCADA_VIEW', 'MODEL_VIEW', 'ALARM_VIEW', 'EVENT_VIEW']::TEXT[],
ADD COLUMN "deviceScopeMode" TEXT NOT NULL DEFAULT 'ALL';

CREATE TABLE "UserDeviceAccess" (
  "userId" TEXT NOT NULL,
  "deviceId" TEXT NOT NULL,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT "UserDeviceAccess_pkey" PRIMARY KEY ("userId", "deviceId")
);

CREATE TABLE "UserDeviceGroupAccess" (
  "userId" TEXT NOT NULL,
  "groupId" TEXT NOT NULL,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT "UserDeviceGroupAccess_pkey" PRIMARY KEY ("userId", "groupId")
);

CREATE INDEX "UserDeviceAccess_deviceId_idx" ON "UserDeviceAccess"("deviceId");
CREATE INDEX "UserDeviceGroupAccess_groupId_idx" ON "UserDeviceGroupAccess"("groupId");

ALTER TABLE "UserDeviceAccess"
ADD CONSTRAINT "UserDeviceAccess_userId_fkey" FOREIGN KEY ("userId") REFERENCES "User"("id") ON DELETE CASCADE ON UPDATE CASCADE,
ADD CONSTRAINT "UserDeviceAccess_deviceId_fkey" FOREIGN KEY ("deviceId") REFERENCES "Device"("id") ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE "UserDeviceGroupAccess"
ADD CONSTRAINT "UserDeviceGroupAccess_userId_fkey" FOREIGN KEY ("userId") REFERENCES "User"("id") ON DELETE CASCADE ON UPDATE CASCADE,
ADD CONSTRAINT "UserDeviceGroupAccess_groupId_fkey" FOREIGN KEY ("groupId") REFERENCES "DeviceGroup"("id") ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE "User" ADD CONSTRAINT "User_deviceScopeMode_check" CHECK ("deviceScopeMode" IN ('ALL', 'CUSTOM'));
