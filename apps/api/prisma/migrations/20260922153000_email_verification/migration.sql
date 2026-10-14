-- Normalize existing addresses before enforcing one account per email.
UPDATE "User"
SET "email" = lower(btrim("email"))
WHERE "email" IS NOT NULL;

CREATE UNIQUE INDEX "User_email_key" ON "User"("email");
