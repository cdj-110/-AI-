ALTER TABLE "FactoryGateway" ADD COLUMN "chipUsid" TEXT;

CREATE UNIQUE INDEX "FactoryGateway_chipUsid_key" ON "FactoryGateway"("chipUsid");
