-- EG100 has no CAN interface. Completing this field makes the product catalog
-- authoritative for license-driven first-boot hardware initialization.
UPDATE "ProductModel"
SET
  "ethernetPortCount" = 2,
  "serialPortCount" = 4,
  "rs232PortCount" = 2,
  "rs485PortCount" = 2,
  "canPortCount" = 0,
  "supports4G" = TRUE,
  "supportsWifi" = TRUE,
  "updatedAt" = CURRENT_TIMESTAMP
WHERE "code" = 'EG100';
