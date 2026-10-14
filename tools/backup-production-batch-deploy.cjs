const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const crypto = require('node:crypto');
const root = path.resolve(__dirname, '..');
const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const dir = path.join(root, 'secrets', `production-batch-deploy-${stamp}`);
fs.mkdirSync(dir, { mode: 0o700 });
const identity = cp.execFileSync('whoami', [], { encoding: 'utf8' }).trim();
cp.execFileSync('icacls', [dir, '/inheritance:r', '/grant:r', `${identity}:(OI)(CI)F`, 'SYSTEM:(OI)(CI)F'], { stdio: 'ignore' });
const docker = args => cp.execFileSync('docker', ['compose', ...args], { cwd: root, maxBuffer: 128 * 1024 * 1024 });
const snapshot = docker(['exec', '-T', 'postgres', 'psql', '-X', '-v', 'ON_ERROR_STOP=1', '-U', 'weikong', '-d', 'weikong_iot', '-At', '-c', `
SELECT json_build_object(
  'factoryCount',(SELECT count(*) FROM "FactoryGateway"),
  'deviceCount',(SELECT count(*) FROM "Device"),
  'usersHash',(SELECT md5(COALESCE(string_agg(row_to_json(t)::text,'' ORDER BY id),'')) FROM "User" t),
  'tenantsHash',(SELECT md5(COALESCE(string_agg(row_to_json(t)::text,'' ORDER BY id),'')) FROM "Tenant" t),
  'factoriesHash',(SELECT md5(COALESCE(string_agg(row_to_json(t)::text,'' ORDER BY id),'')) FROM "FactoryGateway" t)
);`]).toString().trim();
const raw = docker(['exec', '-T', 'postgres', 'pg_dump', '-U', 'weikong', '-d', 'weikong_iot', '-Fc']);
if (!raw.subarray(0, 5).equals(Buffer.from('PGDMP'))) throw new Error('Invalid PostgreSQL backup archive');
const password = fs.readFileSync(path.join(root, 'secrets/authorization-20260916/backup-recovery-password.txt'), 'utf8').trim();
const salt = crypto.randomBytes(16), iv = crypto.randomBytes(12);
const key = crypto.scryptSync(password, salt, 32, { N: 32768, maxmem: 128 * 1024 * 1024 });
const cipher = crypto.createCipheriv('aes-256-gcm', key, iv);
const encrypted = Buffer.concat([cipher.update(raw), cipher.final()]);
const tag = cipher.getAuthTag();
const decipher = crypto.createDecipheriv('aes-256-gcm', key, iv);
decipher.setAuthTag(tag);
if (!Buffer.concat([decipher.update(encrypted), decipher.final()]).equals(raw)) throw new Error('Encrypted backup verification failed');
fs.writeFileSync(path.join(dir, 'platform.dump.aesgcm'), encrypted, { flag: 'wx', mode: 0o600 });
fs.writeFileSync(path.join(dir, 'platform.dump.aesgcm.json'), JSON.stringify({ format: 'aes-256-gcm', kdf: 'scrypt', N: 32768, salt: salt.toString('base64'), iv: iv.toString('base64'), tag: tag.toString('base64'), sha256: crypto.createHash('sha256').update(raw).digest('hex') }), { flag: 'wx', mode: 0o600 });
fs.writeFileSync(path.join(dir, 'before.json'), snapshot, { flag: 'wx', mode: 0o600 });
console.log(JSON.stringify({ backupDirectory: dir, bytes: raw.length, encryptedBackupVerified: true, before: JSON.parse(snapshot) }));
