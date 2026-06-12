CREATE TABLE "OperationLog" (
  "id" TEXT NOT NULL,
  "tenantId" TEXT,
  "userId" TEXT,
  "username" TEXT NOT NULL,
  "module" TEXT NOT NULL,
  "action" TEXT NOT NULL,
  "targetType" TEXT NOT NULL,
  "targetId" TEXT,
  "targetName" TEXT,
  "ip" TEXT,
  "userAgent" TEXT,
  "detail" JSONB,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

  CONSTRAINT "OperationLog_pkey" PRIMARY KEY ("id")
);

CREATE INDEX "OperationLog_tenantId_idx" ON "OperationLog"("tenantId");
CREATE INDEX "OperationLog_userId_idx" ON "OperationLog"("userId");
CREATE INDEX "OperationLog_module_idx" ON "OperationLog"("module");
CREATE INDEX "OperationLog_action_idx" ON "OperationLog"("action");
CREATE INDEX "OperationLog_targetType_idx" ON "OperationLog"("targetType");
CREATE INDEX "OperationLog_createdAt_idx" ON "OperationLog"("createdAt");

ALTER TABLE "OperationLog" ADD CONSTRAINT "OperationLog_tenantId_fkey" FOREIGN KEY ("tenantId") REFERENCES "Tenant"("id") ON DELETE SET NULL ON UPDATE CASCADE;
ALTER TABLE "OperationLog" ADD CONSTRAINT "OperationLog_userId_fkey" FOREIGN KEY ("userId") REFERENCES "User"("id") ON DELETE SET NULL ON UPDATE CASCADE;
