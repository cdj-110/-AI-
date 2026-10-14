import { copyFileSync, readFileSync, writeFileSync, renameSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';
import { execFileSync } from 'node:child_process';
import { constants } from 'node:fs';

const root = resolve(import.meta.dirname, '..');
const envPath = resolve(root, '.env');
const backupPath = resolve(root, 'secrets/https-lan-20260921/dotenv-before-https.env');
const stagedPath = `${envPath}.https-stage`;
const origin = 'https://192.168.1.141:3443';
const required = {
  ACTIVATION_CLOUD_API: { before: 'http://192.168.1.141:3100', after: origin },
  ACTIVATION_REMOTE_ACCESS_ENDPOINT: { before: 'ws://192.168.1.141:3100/api/remote-access/tunnel', after: `wss://192.168.1.141:3443/api/remote-access/tunnel` },
  OTA_PUBLIC_BASE_URL: { before: '', after: origin },
};
let raw = readFileSync(envPath, 'utf8');
const values = Object.fromEntries(raw.split(/\r?\n/).filter(Boolean).map(line => {
  const index = line.indexOf('=');
  return index < 1 ? ['', ''] : [line.slice(0, index), line.slice(index + 1)];
}));
for (const [name, change] of Object.entries(required)) {
  if (values[name] !== change.before) throw new Error(`${name} does not match the verified pre-deployment value`);
}
if (existsSync(backupPath) || existsSync(stagedPath)) throw new Error('backup or staged environment file already exists');
copyFileSync(envPath, backupPath, constants.COPYFILE_EXCL);
for (const [name, change] of Object.entries(required)) {
  const pattern = new RegExp(`^${name}=.*$`, 'm');
  raw = raw.replace(pattern, `${name}=${change.after}`);
}
writeFileSync(stagedPath, raw, { flag: 'wx', mode: 0o600 });
if (process.platform === 'win32') {
  const identity = execFileSync('whoami', [], { encoding: 'utf8' }).trim();
  execFileSync('icacls', [stagedPath, '/inheritance:r', '/grant:r', `${identity}:F`, 'SYSTEM:F'], { stdio: 'ignore' });
}
renameSync(stagedPath, envPath);
console.log(JSON.stringify({ cloudApi: origin, remoteAccessEndpoint: required.ACTIVATION_REMOTE_ACCESS_ENDPOINT.after, otaPublicBaseURL: origin, backupCreated: true }));
