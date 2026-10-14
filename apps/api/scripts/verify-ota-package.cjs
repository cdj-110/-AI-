const { generateKeyPairSync } = require('node:crypto');
const { mkdtempSync, writeFileSync, readFileSync, rmSync } = require('node:fs');
const { tmpdir } = require('node:os');
const { join, resolve } = require('node:path');
const { gzipSync } = require('node:zlib');
const { spawnSync } = require('node:child_process');
const { verifyOtaPackage } = require('../dist/ota/ota-package');
const { assertForwardOtaRelease } = require('../dist/ota/ota-release');

async function main() {
  const temp = mkdtempSync(join(tmpdir(), 'weikong-ota-test-'));
  try {
    const { publicKey, privateKey } = generateKeyPairSync('ed25519');
    const privateRaw = privateKey.export({ type: 'pkcs8', format: 'der' }).toString('base64');
    const publicRaw = publicKey.export({ type: 'spki', format: 'der' }).subarray(-32).toString('base64');
    const artifact = join(temp, 'gateway.gz');
    const output = join(temp, 'gateway.wku');
    writeFileSync(artifact, gzipSync(Buffer.from('test gateway artifact')));
    const signer = resolve(__dirname, '../../../tools/create-eg100-ota-package.mjs');
    const result = spawnSync(process.execPath, [signer, artifact, output, '2026092101', 'test-1'], {
      env: { ...process.env, OTA_SIGNING_PRIVATE_KEY: privateRaw, OTA_SIGNING_KEY_ID: 'fixture' }, encoding: 'utf8',
    });
    if (result.status !== 0) throw new Error(`signer failed: ${result.stderr}`);
    const valid = await verifyOtaPackage(output, { fixture: publicRaw });
    if (valid.manifest.release !== 2026092101) throw new Error('release mismatch');
    let rejected = false;
    try { await verifyOtaPackage(output, { unknown: publicRaw }); } catch { rejected = true; }
    if (!rejected) throw new Error('untrusted key accepted');
    const tampered = readFileSync(output);
    tampered[tampered.length - 1] ^= 1;
    writeFileSync(output, tampered);
    rejected = false;
    try { await verifyOtaPackage(output, { fixture: publicRaw }); } catch { rejected = true; }
    if (!rejected) throw new Error('tampered artifact accepted');
    assertForwardOtaRelease(2026092101n, 2026092102n);
    rejected = false;
    try { assertForwardOtaRelease(2026092101n, 2026092101n); } catch (error) {
      rejected = error?.status === 400 && String(error.message).includes('2026092101');
    }
    if (!rejected) throw new Error('duplicate OTA release accepted by platform preflight');
    rejected = false;
    try { assertForwardOtaRelease(2026092101n, 2026092100n); } catch (error) { rejected = error?.status === 400; }
    if (!rejected) throw new Error('downgrade OTA release accepted by platform preflight');
    console.log('OTA package signing, verification and tampering checks passed');
  } finally { rmSync(temp, { recursive: true, force: true }); }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
