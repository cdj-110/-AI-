-- No existing heartbeat is treated as proof of activation.
ALTER TABLE "FactoryGateway"
  ADD COLUMN "issuedLicense" JSONB,
  ADD COLUMN "reportedLicense" JSONB,
  ADD COLUMN "licenseActivatedAt" TIMESTAMP(3),
  ADD COLUMN "licenseReportedAt" TIMESTAMP(3);
