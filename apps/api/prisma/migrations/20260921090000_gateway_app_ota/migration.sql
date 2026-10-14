CREATE TABLE "OtaPackage" (
  "id" TEXT NOT NULL,
  "model" TEXT NOT NULL,
  "release" BIGINT NOT NULL,
  "version" TEXT NOT NULL,
  "keyId" TEXT NOT NULL,
  "artifactSha256" TEXT NOT NULL,
  "packageSha256" TEXT NOT NULL,
  "fileName" TEXT NOT NULL,
  "size" INTEGER NOT NULL,
  "createdBy" TEXT NOT NULL,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT "OtaPackage_pkey" PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX "OtaPackage_packageSha256_key" ON "OtaPackage"("packageSha256");
CREATE UNIQUE INDEX "OtaPackage_fileName_key" ON "OtaPackage"("fileName");
CREATE INDEX "OtaPackage_model_release_idx" ON "OtaPackage"("model", "release");

CREATE TABLE "OtaTask" (
  "id" TEXT NOT NULL,
  "gatewayId" TEXT NOT NULL,
  "packageId" TEXT NOT NULL,
  "status" TEXT NOT NULL DEFAULT 'PENDING',
  "message" TEXT,
  "createdBy" TEXT NOT NULL,
  "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "expiresAt" TIMESTAMP(3) NOT NULL,
  "completedAt" TIMESTAMP(3),
  CONSTRAINT "OtaTask_pkey" PRIMARY KEY ("id")
);
CREATE INDEX "OtaTask_gatewayId_createdAt_idx" ON "OtaTask"("gatewayId", "createdAt");
CREATE INDEX "OtaTask_status_expiresAt_idx" ON "OtaTask"("status", "expiresAt");
CREATE UNIQUE INDEX "OtaTask_one_active_gateway_idx" ON "OtaTask"("gatewayId") WHERE "status" IN ('PENDING', 'SENT', 'INSTALLING');
ALTER TABLE "OtaTask" ADD CONSTRAINT "OtaTask_gatewayId_fkey" FOREIGN KEY ("gatewayId") REFERENCES "FactoryGateway"("id") ON DELETE RESTRICT ON UPDATE CASCADE;
ALTER TABLE "OtaTask" ADD CONSTRAINT "OtaTask_packageId_fkey" FOREIGN KEY ("packageId") REFERENCES "OtaPackage"("id") ON DELETE RESTRICT ON UPDATE CASCADE;
