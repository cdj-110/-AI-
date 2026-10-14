ALTER TABLE "FactoryGateway" ADD COLUMN "activationSecretEncrypted" TEXT;
ALTER TABLE "FactoryGateway" ADD COLUMN "licenseSequence" INTEGER NOT NULL DEFAULT 0;
ALTER TABLE "FactoryGateway" ADD CONSTRAINT "FactoryGateway_licenseSequence_nonnegative" CHECK ("licenseSequence" >= 0);
