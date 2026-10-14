// Offline release tool. Private OTA signing keys must never be copied to a
// gateway, cloud API container, repository or upgrade package.
import { createHash, createPrivateKey, sign } from 'node:crypto';
import { createReadStream, createWriteStream, promises as fs } from 'node:fs';
import { pipeline } from 'node:stream/promises';

const [artifact, output, releaseText, version] = process.argv.slice(2);
const release = Number(releaseText);
const keyId = process.env.OTA_SIGNING_KEY_ID || '';
const privateKeyValue = process.env.OTA_SIGNING_PRIVATE_KEY || '';
if (!artifact || !output || !Number.isSafeInteger(release) || release < 1 || release > 999999999999 || !version || version.length > 64 || !/^[a-zA-Z0-9._-]+$/.test(version) || !keyId || !privateKeyValue) {
  throw new Error('用法：设置 OTA_SIGNING_PRIVATE_KEY、OTA_SIGNING_KEY_ID，再传入 <gateway.gz> <output.wku> <递增发布序号> <版本号>');
}
const file = await fs.open(artifact, 'r');
const magic = Buffer.alloc(2);
try { await file.read(magic, 0, 2, 0); } finally { await file.close(); }
if (magic[0] !== 0x1f || magic[1] !== 0x8b) throw new Error('输入文件必须是 gzip 压缩的 EG100 可执行程序');
const digest = createHash('sha256');
let size = 0;
for await (const chunk of createReadStream(artifact)) { digest.update(chunk); size += chunk.length; }
if (size < 1 || size > 64 * 1024 * 1024) throw new Error('压缩程序不能超过 64 MiB');
const manifest = { formatVersion: 1, model: 'EG100', release, version, artifactSha256: digest.digest('hex'), artifactSize: size, keyId };
const signed = Buffer.from(`WKU1\n${manifest.formatVersion}\n${manifest.model}\n${manifest.release}\n${manifest.version}\n${manifest.artifactSha256}\n${manifest.artifactSize}\n${manifest.keyId}\n`);
const key = createPrivateKey({ key: Buffer.from(privateKeyValue, 'base64'), format: 'der', type: 'pkcs8' });
const complete = { ...manifest, signature: sign(null, signed, key).toString('base64') };
const raw = Buffer.from(JSON.stringify(complete));
if (raw.length > 4096) throw new Error('升级包清单过长');
const header = Buffer.alloc(8);
header.write('WKU1', 0, 'ascii');
header.writeUInt32BE(raw.length, 4);
const target = createWriteStream(output, { flags: 'wx', mode: 0o600 });
target.write(header);
target.write(raw);
await pipeline(createReadStream(artifact), target);
console.log(JSON.stringify({ output, model: manifest.model, release, version, artifactSha256: manifest.artifactSha256, size }));
