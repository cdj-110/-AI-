ALTER TABLE "ProductModel"
ADD COLUMN "rs232PortCount" INTEGER,
ADD COLUMN "rs485PortCount" INTEGER,
ADD COLUMN "canPortCount" INTEGER,
ADD CONSTRAINT "ProductModel_typed_ports_check" CHECK (
  ("rs232PortCount" IS NULL OR "rs232PortCount" BETWEEN 0 AND 64) AND
  ("rs485PortCount" IS NULL OR "rs485PortCount" BETWEEN 0 AND 64) AND
  ("canPortCount" IS NULL OR "canPortCount" BETWEEN 0 AND 64)
);

-- Keep other models' old totals intact, without guessing electrical interfaces.
-- EG100 capabilities were explicitly confirmed by the product owner.
UPDATE "ProductModel" SET
"rs232PortCount" = 2, "rs485PortCount" = 2, "serialPortCount" = 4,
"supports4G" = true, "supportsWifi" = true,
"description" = CASE
  WHEN "description" = '网口和串口按现有 EG100 资源登记；4G、WiFi 支持情况待确认，规格不会自动改动已部署网关配置'
  THEN '2 路 RS232、2 路 RS485，支持 WiFi 和 4G；CAN 数量待确认。规格不会自动改动已部署网关配置'
  ELSE "description" END,
"updatedAt" = CURRENT_TIMESTAMP
WHERE "code" = 'EG100';
