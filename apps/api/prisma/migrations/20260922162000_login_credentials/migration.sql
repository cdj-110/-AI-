-- 用户名改为可重复的展示名称；邮箱和手机号作为唯一登录凭证。
UPDATE "User" SET "email" = lower(trim("email")) WHERE "email" IS NOT NULL;
UPDATE "User" SET "phone" = NULLIF(trim("phone"), '') WHERE "phone" IS NOT NULL;

DROP INDEX IF EXISTS "User_username_key";
CREATE UNIQUE INDEX "User_phone_key" ON "User"("phone");
