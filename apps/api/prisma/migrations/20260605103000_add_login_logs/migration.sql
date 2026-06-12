CREATE TABLE "LoginLog" (
  "id" TEXT NOT NULL,
  "tenantId" TEXT,
  "userId" TEXT,
  "username" TEXT NOT NULL,
  "ip" TEXT NOT NULL,
  "userAgent" TEXT,
  "success" BOOLEAN NOT NULL DEFAULT true,
  "reason" TEXT,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

  CONSTRAINT "LoginLog_pkey" PRIMARY KEY ("id")
);

CREATE INDEX "LoginLog_tenantId_idx" ON "LoginLog"("tenantId");
CREATE INDEX "LoginLog_userId_idx" ON "LoginLog"("userId");
CREATE INDEX "LoginLog_ip_idx" ON "LoginLog"("ip");
CREATE INDEX "LoginLog_createdAt_idx" ON "LoginLog"("createdAt");

ALTER TABLE "LoginLog" ADD CONSTRAINT "LoginLog_tenantId_fkey" FOREIGN KEY ("tenantId") REFERENCES "Tenant"("id") ON DELETE SET NULL ON UPDATE CASCADE;
ALTER TABLE "LoginLog" ADD CONSTRAINT "LoginLog_userId_fkey" FOREIGN KEY ("userId") REFERENCES "User"("id") ON DELETE SET NULL ON UPDATE CASCADE;
