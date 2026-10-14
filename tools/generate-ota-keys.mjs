import { generateKeyPairSync, randomBytes, createHash } from 'node:crypto';
import { existsSync, mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { execFileSync } from 'node:child_process';

const directory = resolve(process.argv[2] || 'secrets/ota-20260921');
const keyId = process.argv[3] || 'eg100-ota-20260921';
if (!/^[A-Za-z0-9._-]{1,64}$/.test(keyId)) throw new Error('Invalid OTA key ID');
if (existsSync(directory)) throw new Error('Refusing to overwrite existing OTA key directory');
mkdirSync(directory, { recursive: true, mode: 0o700 });
if (process.platform === 'win32') {
  const identity = execFileSync('whoami', [], { encoding: 'utf8' }).trim();
  execFileSync('icacls', [directory, '/inheritance:r', '/grant:r', `${identity}:(OI)(CI)F`, 'SYSTEM:(OI)(CI)F'], { stdio: 'ignore' });
}
const { privateKey, publicKey } = generateKeyPairSync('ed25519');
const privateDer = privateKey.export({ format: 'der', type: 'pkcs8' });
const publicRaw = publicKey.export({ format: 'der', type: 'spki' }).subarray(-32);
const ring = JSON.stringify({ [keyId]: publicRaw.toString('base64') });
const secret = randomBytes(48).toString('base64url');
writeFileSync(resolve(directory, 'ota-private-key.env'), `OTA_SIGNING_KEY_ID=${keyId}\nOTA_SIGNING_PRIVATE_KEY=${privateDer.toString('base64')}\n`, { flag: 'wx', mode: 0o600 });
writeFileSync(resolve(directory, 'ota-public-key.env'), `OTA_TRUSTED_PUBLIC_KEYS='${ring}'\nWK_OTA_TRUSTED_PUBLIC_KEYS='${ring}'\n`, { flag: 'wx', mode: 0o600 });
writeFileSync(resolve(directory, 'ota-download-secret.env'), `OTA_DOWNLOAD_SECRET=${secret}\n`, { flag: 'wx', mode: 0o600 });
console.log(JSON.stringify({ directory, keyId, publicKeyFingerprint: createHash('sha256').update(publicRaw).digest('hex').slice(0, 16), privateKeyStoredLocally: true, publicKeyBytes: publicRaw.length, downloadSecretConfigured: true }));
