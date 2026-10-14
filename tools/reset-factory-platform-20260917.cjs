// One-off user-authorized initialization. No global truncate or Redis flush.
const fs = require('node:fs'), fsp = fs.promises, path = require('node:path');
const cp = require('node:child_process'), crypto = require('node:crypto');
const { pipeline } = require('node:stream/promises');
const root = path.resolve(__dirname, '..');
const mode = process.argv[2];
const dir = path.resolve(root, process.argv[3] || 'invalid');
if (!dir.startsWith(path.join(root, 'secrets', 'initialization-20260917-'))) throw new Error('Invalid scoped backup path');

function run(args, input, opts = {}) {
  return new Promise((resolve, reject) => {
    const child = cp.spawn('docker', ['compose', ...args], { cwd: root, stdio: ['pipe', 'pipe', 'pipe'] });
    let out = '', err = '';
    child.stderr.on('data', chunk => { err += chunk; });
    let inputStream;
    child.stdin.on('error', error => { if (error.code === 'EPIPE') inputStream?.destroy(); else reject(error); });
    const written = opts.file ? pipeline(child.stdout, fs.createWriteStream(opts.file, { flags: 'wx', mode: 0o600 })) : Promise.resolve();
    if (!opts.file) child.stdout.on('data', chunk => { out += chunk; });
    written.catch(reject);
    child.on('error', reject);
    child.on('close', code => code ? reject(new Error(`Docker ${args[0]} failed: ${err.slice(0, 600)}`)) : written.then(() => resolve(out.trim()), reject));
    if (opts.inputFile) { inputStream = fs.createReadStream(opts.inputFile); inputStream.on('error', reject); inputStream.pipe(child.stdin); }
    else child.stdin.end(input || '');
  });
}
const sql = (service, database, query) => run(['exec', '-T', service, 'psql', '-X', '-v', 'ON_ERROR_STOP=1', '-U', 'weikong', '-d', database, '-At'], query);
const literal = value => "'" + value.replace(/'/g, "''") + "'";
async function snapshot() {
  const factories = JSON.parse(await sql('postgres', 'weikong_iot', 'SELECT COALESCE(json_agg(row_to_json(t) ORDER BY id),\'[]\'::json) FROM (SELECT id,sn,"deviceId" FROM "FactoryGateway") t;'));
  const devices = JSON.parse(await sql('postgres', 'weikong_iot', 'SELECT COALESCE(json_agg(row_to_json(t) ORDER BY id),\'[]\'::json) FROM (SELECT id,"deviceKey","gatewayId","tenantId" FROM "Device") t;'));
  const ids = new Set(factories.map(f => f.deviceId).filter(Boolean));
  for (;;) { const before = ids.size; for (const d of devices) if (ids.has(d.gatewayId)) ids.add(d.id); if (before === ids.size) break; }
  return { factories, targets: devices.filter(d => ids.has(d.id)), otherDevices: devices.filter(d => !ids.has(d.id)),
    preserved: await sql('postgres', 'weikong_iot', 'SELECT md5(COALESCE(string_agg(row_to_json(t)::text,\'\' ORDER BY id),\'\')) FROM "User" t; SELECT md5(COALESCE(string_agg(row_to_json(t)::text,\'\' ORDER BY id),\'\')) FROM "Tenant" t;') };
}
async function hashFile(file) { const hash = crypto.createHash('sha256'); for await (const part of fs.createReadStream(file)) hash.update(part); return hash.digest('hex'); }
async function protectBackup(name) {
  const file = path.join(dir, name), encrypted = file + '.aesgcm';
  const password = await fsp.readFile(path.join(root, 'secrets/authorization-20260916/backup-recovery-password.txt'));
  const salt = crypto.randomBytes(16), iv = crypto.randomBytes(12);
  const key = crypto.scryptSync(password.toString().trim(), salt, 32, { N: 32768, maxmem: 128 * 1024 * 1024 });
  const cipher = crypto.createCipheriv('aes-256-gcm', key, iv), sha256 = await hashFile(file);
  await pipeline(fs.createReadStream(file), cipher, fs.createWriteStream(encrypted, { flags: 'wx', mode: 0o600 }));
  const tag = cipher.getAuthTag(), decipher = crypto.createDecipheriv('aes-256-gcm', key, iv); decipher.setAuthTag(tag);
  const check = crypto.createHash('sha256');
  const stream = fs.createReadStream(encrypted).pipe(decipher);
  for await (const part of stream) check.update(part);
  if (check.digest('hex') !== sha256) throw new Error('Encrypted backup failed verification');
  await fsp.writeFile(encrypted + '.json', JSON.stringify({ format: 'aes-256-gcm', kdf: 'scrypt', N: 32768, salt: salt.toString('base64'), iv: iv.toString('base64'), tag: tag.toString('base64'), sha256, recoveryPasswordFile: '../authorization-20260916/backup-recovery-password.txt' }), { mode: 0o600, flag: 'wx' });
  const encryptedSha256 = await hashFile(encrypted);
  await fsp.unlink(file); // Verified encrypted replacement for this newly generated dump only.
  return { name: name + '.aesgcm', sha256, encryptedSha256 };
}

(async () => {
  if (mode === 'seal-backups') {
    for (const name of ['platform.dump', 'telemetry.dump']) if (fs.existsSync(path.join(dir, name))) console.log(JSON.stringify({ sealed: name, ...(await protectBackup(name)) }));
    return;
  }
  if (!['backup', 'clear'].includes(mode)) throw new Error('Expected backup or clear');
  let stopped = false;
  try {
    if (mode === 'backup') {
      await fsp.mkdir(dir, { recursive: false, mode: 0o700 });
      const identity = cp.execFileSync('whoami', [], { encoding: 'utf8' }).trim();
      cp.execFileSync('icacls', [dir, '/inheritance:r', '/grant:r', `${identity}:(OI)(CI)F`, 'SYSTEM:(OI)(CI)F'], { stdio: 'ignore' });
    }
    await run(['stop', 'api', 'ingest']); stopped = true;
    if (mode === 'backup') {
      const plan = await snapshot();
      const backups = [];
      for (const [service, db, name, expected] of [['postgres', 'weikong_iot', 'platform.dump', 'FactoryGateway'], ['timescaledb', 'weikong_ts', 'telemetry.dump', 'telemetry_events']]) {
        await run(['exec', '-T', service, 'pg_dump', '-U', 'weikong', '-d', db, '-Fc'], '', { file: path.join(dir, name) });
        const contents = await run(['exec', '-T', service, 'pg_restore', '--list'], '', { inputFile: path.join(dir, name) });
        if (!contents.includes(expected)) throw new Error('Backup archive has no expected data table');
        backups.push(await protectBackup(name));
        console.log(JSON.stringify({ stage: 'backup-verified', database: db, encrypted: true }));
      }
      await fsp.writeFile(path.join(dir, 'plan.json'), JSON.stringify({ ...plan, backups, createdAt: new Date().toISOString() }, null, 2), { flag: 'wx', mode: 0o600 });
      console.log(JSON.stringify({ backupDirectory: dir, factoryCount: plan.factories.length, associatedDevices: plan.targets.length, unrelatedDevicesPreserved: plan.otherDevices.length, backupsVerified: true }));
    } else {
      const plan = JSON.parse(await fsp.readFile(path.join(dir, 'plan.json'), 'utf8'));
      const current = await snapshot();
      for (const name of ['factories', 'targets', 'otherDevices', 'preserved']) if (JSON.stringify(current[name]) !== JSON.stringify(plan[name])) throw new Error('Scope changed since backup; abort');
      for (const item of plan.backups) if (await hashFile(path.join(dir, item.name)) !== item.encryptedSha256) throw new Error('Backup checksum changed');
      const ids = plan.targets.map(d => d.id), list = ids.length ? ids.map(literal).join(',') : "'no-targets'";
      const unrelatedBefore = await sql('timescaledb', 'weikong_ts', `SELECT count(*) FROM telemetry_events WHERE device_id NOT IN (${list});`);
      const batches = JSON.parse(await sql('timescaledb', 'weikong_ts', `SELECT COALESCE(json_agg(row_to_json(t) ORDER BY start),'[]'::json) FROM (
        SELECT time_bucket('1 hour',time)::text AS start, (time_bucket('1 hour',time)+interval '1 hour')::text AS finish, count(*)::integer AS count
        FROM telemetry_events WHERE device_id IN (${list}) GROUP BY 1,2) t;`));
      let telemetry = 0;
      for (let index = 0; index < batches.length; index++) {
        const batch = batches[index];
        const output = await sql('timescaledb', 'weikong_ts', `SET statement_timeout='90s'; SET work_mem='16MB'; DELETE FROM telemetry_events WHERE device_id IN (${list}) AND time >= ${literal(batch.start)}::timestamptz AND time < ${literal(batch.finish)}::timestamptz;`);
        const matched = output.match(/DELETE (\d+)/);
        if (!matched || Number(matched[1]) !== batch.count) throw new Error('Telemetry batch count changed');
        telemetry += Number(matched[1]);
        await fsp.appendFile(path.join(dir, 'telemetry-deletion-progress.jsonl'), JSON.stringify({ batch:index+1, totalBatches:batches.length, removed:Number(matched[1]), totalRemoved:telemetry, time:batch.start })+'\n', { mode:0o600 });
        if ((index+1)%10===0 || index===batches.length-1) console.log(JSON.stringify({ stage:'telemetry-cleanup', batchesCompleted:index+1, totalBatches:batches.length, removedRows:telemetry }));
      }
      const deleted = await sql('postgres', 'weikong_iot', `BEGIN; LOCK TABLE "FactoryGateway", "Device" IN ACCESS EXCLUSIVE MODE;
        DELETE FROM "DeviceLog" WHERE "deviceId" IN (${list});
        DELETE FROM "GatewayEvent" WHERE "gatewayId" IN (${list}) OR "deviceId" IN (${list});
        DELETE FROM "Device" WHERE id IN (${list});
        DELETE FROM "FactoryGateway";
        COMMIT;`);
      const keys = (await run(['exec', '-T', 'redis', 'redis-cli', '--scan', '--pattern', 'weikong:remote-access:*'])).split(/\r?\n/).filter(Boolean);
      let removedSessions = 0;
      for (const key of keys) {
        if (!/^weikong:remote-access:(session|token):[\w-]+$/.test(key)) continue;
        const raw = await run(['exec', '-T', 'redis', 'redis-cli', '--raw', 'GET', key]);
        let session; try { session = JSON.parse(raw); } catch { continue; }
        if (ids.includes(session.deviceId)) { await run(['exec', '-T', 'redis', 'redis-cli', 'DEL', key]); removedSessions++; }
      }
      const after = await snapshot();
      if (after.factories.length || after.targets.length || JSON.stringify(after.otherDevices) !== JSON.stringify(plan.otherDevices) || after.preserved !== plan.preserved) throw new Error('Post-reset scope verification failed');
      if (await sql('timescaledb', 'weikong_ts', `SELECT count(*) FROM telemetry_events WHERE device_id IN (${list});`) !== '0') throw new Error('Associated telemetry remains');
      if (await sql('timescaledb', 'weikong_ts', `SELECT count(*) FROM telemetry_events WHERE device_id NOT IN (${list});`) !== unrelatedBefore) throw new Error('Unrelated telemetry changed');
      const result = { factoryRecordsRemoved: plan.factories.length, associatedDevicesRemoved: ids.length, associatedTelemetryDeletion: telemetry, postgresDeletion: deleted, unrelatedTelemetryRowsPreserved: Number(unrelatedBefore), accountAndTenantHashesUnchanged: true, unrelatedDevicesPreserved: after.otherDevices.length, removedRemoteSessionKeys: removedSessions, completedAt: new Date().toISOString() };
      await fsp.writeFile(path.join(dir, 'result.json'), JSON.stringify(result, null, 2), { mode: 0o600, flag: 'wx' });
      console.log(JSON.stringify(result));
    }
  } finally { if (stopped) await run(['start', 'api', 'ingest']); }
})().catch(error => { console.error(error.message); process.exitCode = 1; });
