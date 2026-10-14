CREATE TABLE "ProductModel" (
    "code" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "ethernetPortCount" INTEGER,
    "serialPortCount" INTEGER,
    "supports4G" BOOLEAN,
    "supportsWifi" BOOLEAN,
    "enabled" BOOLEAN NOT NULL DEFAULT true,
    "description" TEXT,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,
    CONSTRAINT "ProductModel_pkey" PRIMARY KEY ("code"),
    CONSTRAINT "ProductModel_port_counts_check" CHECK (
      ("ethernetPortCount" IS NULL OR "ethernetPortCount" BETWEEN 0 AND 64) AND
      ("serialPortCount" IS NULL OR "serialPortCount" BETWEEN 0 AND 64)
    )
);

-- Keep all existing model references valid without guessing unknown capabilities.
INSERT INTO "ProductModel" ("code", "name", "description", "updatedAt")
SELECT DISTINCT "model", "model", '从出厂网关档案迁移，硬件规格待确认', CURRENT_TIMESTAMP
FROM "FactoryGateway";

-- EG100's currently verified physical resources: two Ethernet and two serial ports.
-- Optional wireless modules must be confirmed by an administrator.
INSERT INTO "ProductModel" ("code", "name", "ethernetPortCount", "serialPortCount", "description", "updatedAt")
VALUES ('EG100', 'EG100 边缘网关', 2, 2, '网口和串口按现有 EG100 资源登记；4G、WiFi 支持情况待确认，规格不会自动改动已部署网关配置', CURRENT_TIMESTAMP)
ON CONFLICT ("code") DO NOTHING;

ALTER TABLE "FactoryGateway" ADD CONSTRAINT "FactoryGateway_model_fkey"
FOREIGN KEY ("model") REFERENCES "ProductModel"("code") ON DELETE RESTRICT ON UPDATE RESTRICT;
