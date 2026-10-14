ALTER TABLE "FactoryGateway" ADD COLUMN "model" TEXT NOT NULL DEFAULT 'EG100';

CREATE TABLE "FactoryProductionCounter" (
    "prefix" TEXT NOT NULL,
    "lastSequence" INTEGER NOT NULL,
    CONSTRAINT "FactoryProductionCounter_pkey" PRIMARY KEY ("prefix")
);
