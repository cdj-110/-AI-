// One-time local maintenance issuance for the already registered EG100.
// Never print the signed file or the recoverable device secret.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const root = path.resolve(__dirname, '..');
const api = path.join(root, 'apps/api');
const { PrismaClient } = require(path.join(api, 'node_modules/@prisma/client'));
const { ConfigService } = require(path.join(api, 'node_modules/@nestjs/config'));
const dotenv = require(path.join(api, 'node_modules/dotenv'));
const { FactoryGatewaysService } = require(path.join(api, 'dist/factory-gateways/factory-gateways.service.js'));

const sn = 'WK20265AF2F5C6';
const id = 'c5312e88-a27e-4325-b333-b761e2678bc4';
const beforeSequence = 8;
const origin = 'https://192.168.1.141:3443';
const outputPath = path.join(root, 'secrets/https-lan-20260921', `activation-${sn}-seq9.lic`);

async function main() {
  const settings = dotenv.parse(fs.readFileSync(path.join(root, '.env')));
  for (const key of ['ACTIVATION_SIGNING_PRIVATE_KEY', 'ACTIVATION_SIGNING_KEY_ID', 'FACTORY_CREDENTIAL_ENCRYPTION_KEY', 'ACTIVATION_CLOUD_API', 'ACTIVATION_REMOTE_ACCESS_ENDPOINT', 'MQTT_PUBLIC_HOST', 'MQTT_PUBLIC_PORT']) {
    if (settings[key] !== undefined) process.env[key] = settings[key];
  }
  process.env.ACTIVATION_LICENSE_DAYS = '0';
  if (settings.ACTIVATION_CLOUD_API !== origin || settings.ACTIVATION_REMOTE_ACCESS_ENDPOINT !== `wss://192.168.1.141:3443/api/remote-access/tunnel`) throw new Error('HTTPS issuance settings are not ready');
  if (fs.existsSync(outputPath)) throw new Error('refusing to overwrite existing signed license');
  const prisma = new PrismaClient({ datasources: { db: { url: 'postgresql://weikong@127.0.0.1:5432/weikong_iot?schema=public' } } });
  try {
    const current = await prisma.factoryGateway.findUnique({ where: { id } });
    const issued = current?.issuedLicense;
    const reported = current?.reportedLicense;
    if (!current || current.sn !== sn || current.status !== 'BOUND' || current.licenseSequence !== beforeSequence || issued?.licenseSequence !== beforeSequence || reported?.licenseSequence !== beforeSequence || !current.activationSecretEncrypted) {
      throw new Error('gateway authorization preflight changed; no new license issued');
    }
    const service = new FactoryGatewaysService(prisma, new ConfigService());
    const next = await service.resetActivation(id);
    const license = next.activationFile;
    if (license.licenseSequence !== beforeSequence + 1 || license.cloudApi !== origin || license.expiresAt !== '' || !license.signature) {
      throw new Error('issued license has unexpected sequence, HTTPS address or lifetime');
    }
    fs.writeFileSync(outputPath, JSON.stringify(license, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
    await prisma.operationLog.create({ data: {
      username: '本机维护工具', module: '出厂网关', action: '重新生成激活文件（HTTPS 迁移）', targetType: 'FactoryGateway', targetId: id, targetName: sn,
      detail: { oldSequence: beforeSequence, newSequence: license.licenseSequence, cloudApi: origin, licenseId: crypto.createHash('sha256').update(license.signature).digest('hex') },
    } });
    console.log(JSON.stringify({ sn, licenseSequence: license.licenseSequence, cloudApi: license.cloudApi, expiresAt: '永久有效', outputPath, sha256: crypto.createHash('sha256').update(fs.readFileSync(outputPath)).digest('hex') }));
  } finally {
    await prisma.$disconnect();
  }
}
main().catch(error => { console.error(error.message); process.exitCode = 1; });
