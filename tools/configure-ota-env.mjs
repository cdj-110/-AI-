import { copyFileSync, readFileSync, writeFileSync, constants } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(fileURLToPath(new URL('.', import.meta.url)), '..');
const envPath = resolve(root, '.env');
const secretDir = resolve(root, 'secrets/ota-20260921');
const publicText = readFileSync(resolve(secretDir, 'ota-public-key.env'), 'utf8');
const downloadText = readFileSync(resolve(secretDir, 'ota-download-secret.env'), 'utf8');
const ringLine = publicText.split(/\r?\n/).find(line => line.startsWith('OTA_TRUSTED_PUBLIC_KEYS='));
const secretLine = downloadText.split(/\r?\n/).find(line => line.startsWith('OTA_DOWNLOAD_SECRET='));
if (!ringLine || !secretLine) throw new Error('OTA key material is incomplete');
const ring = ringLine.slice('OTA_TRUSTED_PUBLIC_KEYS='.length).replace(/^'|'$/g, '');
if (!JSON.parse(ring)['eg100-ota-20260921']) throw new Error('Unexpected OTA key ring');
const secret = secretLine.slice('OTA_DOWNLOAD_SECRET='.length);
if (secret.length < 32) throw new Error('OTA download secret is too short');
const original = readFileSync(envPath, 'utf8');
const newline = original.includes('\r\n') ? '\r\n' : '\n';
const lines = original.split(/\r?\n/);
function set(key, value) {
  const indices = lines.flatMap((line, index) => line.startsWith(`${key}=`) ? [index] : []);
  if (indices.length > 1) throw new Error(`Duplicate ${key} in .env`);
  if (indices.length === 1) lines[indices[0]] = `${key}=${value}`;
  else lines.push(`${key}=${value}`);
}
set('OTA_TRUSTED_PUBLIC_KEYS', `'${ring}'`);
set('OTA_DOWNLOAD_SECRET', secret);
// No HTTPS endpoint exists yet; leaving this blank disables remote issuance.
set('OTA_PUBLIC_BASE_URL', '');
const backup = resolve(secretDir, 'dotenv-before-ota.env');
copyFileSync(envPath, backup, constants.COPYFILE_EXCL);
writeFileSync(envPath, lines.join(newline), { encoding: 'utf8' });
console.log(JSON.stringify({ configuredPublicKey: true, configuredDownloadSecret: true, remoteIssuanceEnabled: false, backup }));
