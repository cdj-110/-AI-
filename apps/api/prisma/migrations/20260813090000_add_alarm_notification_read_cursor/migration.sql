ALTER TABLE "User" ADD COLUMN "alarmNotificationsReadAt" TIMESTAMP(3);
UPDATE "User" SET "alarmNotificationsReadAt" = CURRENT_TIMESTAMP;
