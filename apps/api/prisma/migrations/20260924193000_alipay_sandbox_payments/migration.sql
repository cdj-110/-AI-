ALTER TABLE "SubscriptionOrder"
  ADD COLUMN "paymentAmountCents" INTEGER,
  ADD COLUMN "paymentMethod" VARCHAR(16) NOT NULL DEFAULT 'MANUAL',
  ADD COLUMN "paymentStatus" VARCHAR(32) NOT NULL DEFAULT 'UNPAID',
  ADD COLUMN "providerTradeNo" VARCHAR(64),
  ADD COLUMN "paidAt" TIMESTAMP(3),
  ADD COLUMN "paymentFailureReason" VARCHAR(500);

CREATE UNIQUE INDEX "SubscriptionOrder_providerTradeNo_key" ON "SubscriptionOrder"("providerTradeNo");
CREATE INDEX "SubscriptionOrder_paymentStatus_requestedAt_idx" ON "SubscriptionOrder"("paymentStatus", "requestedAt");
