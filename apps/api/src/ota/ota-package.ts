import { createHash, createPublicKey, verify } from 'node:crypto';
import { createReadStream, promises as fs } from 'node:fs';

export interface OtaManifest {
  formatVersion: number;
  model: string;
  release: number;
  version: string;
  artifactSha256: string;
  artifactSize: number;
  keyId: string;
  signature: string;
}

export const MAX_OTA_SIZE = 64 * 1024 * 1024;

export function signedPayload(manifest: OtaManifest): Buffer {
  return Buffer.from(`WKU1\n${manifest.formatVersion}\n${manifest.model}\n${manifest.release}\n${manifest.version}\n${manifest.artifactSha256}\n${manifest.artifactSize}\n${manifest.keyId}\n`);
}

async function hashFile(path: string, start = 0): Promise<{ digest: string; size: number }> {
  const hash = createHash('sha256');
  let size = 0;
  for await (const chunk of createReadStream(path, { start })) {
    const bytes = chunk as Buffer;
    hash.update(bytes);
    size += bytes.length;
    if (size > MAX_OTA_SIZE + 4096 + 8) throw new Error('升级包超过 64 MiB 限制');
  }
  return { digest: hash.digest('hex'), size };
}

export async function verifyOtaPackage(path: string, trustedKeys: Record<string, string>) {
  const file = await fs.open(path, 'r');
  let manifest: OtaManifest;
  let offset: number;
  try {
    const header = Buffer.alloc(8);
    if ((await file.read(header, 0, 8, 0)).bytesRead !== 8 || header.toString('ascii', 0, 4) !== 'WKU1') throw new Error('升级包格式无效');
    const length = header.readUInt32BE(4);
    if (length < 1 || length > 4096) throw new Error('升级包清单长度无效');
    const raw = Buffer.alloc(length);
    if ((await file.read(raw, 0, length, 8)).bytesRead !== length) throw new Error('升级包清单不完整');
    manifest = JSON.parse(raw.toString('utf8')) as OtaManifest;
    offset = 8 + length;
  } finally { await file.close(); }
  if (manifest.formatVersion !== 1 || manifest.model !== 'EG100' || !Number.isSafeInteger(manifest.release) || manifest.release < 1 || manifest.release > 999999999999 ||
      !manifest.version || manifest.version.length > 64 || !Number.isSafeInteger(manifest.artifactSize) || manifest.artifactSize < 1 || manifest.artifactSize > MAX_OTA_SIZE ||
      !/^[0-9a-f]{64}$/.test(manifest.artifactSha256) || !manifest.keyId || !manifest.signature) throw new Error('升级包元数据无效');
  const rawKey = Buffer.from(trustedKeys[manifest.keyId] ?? '', 'base64');
  const signature = Buffer.from(manifest.signature, 'base64');
  if (rawKey.length !== 32 || signature.length !== 64) throw new Error('升级包签名密钥未受信任');
  const key = createPublicKey({ key: Buffer.concat([Buffer.from('302a300506032b6570032100', 'hex'), rawKey]), format: 'der', type: 'spki' });
  if (!verify(null, signedPayload(manifest), key, signature)) throw new Error('升级包签名校验失败');
  const artifact = await hashFile(path, offset);
  if (artifact.size !== manifest.artifactSize || artifact.digest !== manifest.artifactSha256) throw new Error('升级程序摘要不符');
  const whole = await hashFile(path);
  return { manifest, packageSha256: whole.digest, size: whole.size };
}
