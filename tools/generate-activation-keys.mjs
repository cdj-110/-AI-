import { generateKeyPairSync } from 'node:crypto';
import { chmodSync, existsSync, mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

const outputDirectory = resolve(process.argv[2] || 'secrets/activation');
const keyId = process.argv[3] || `activation-${new Date().toISOString().slice(0,10)}`;
if (!/^[A-Za-z0-9._-]{1,64}$/.test(keyId)) throw new Error('Invalid keyId');
mkdirSync(outputDirectory, { recursive: true });

const { privateKey, publicKey } = generateKeyPairSync('ed25519');
const privateDer = privateKey.export({ format: 'der', type: 'pkcs8' });
const publicDer = publicKey.export({ format: 'der', type: 'spki' });
const privatePath = resolve(outputDirectory, 'activation-private-key.env');
const publicPath = resolve(outputDirectory, 'activation-public-key.env');
if (existsSync(privatePath) || existsSync(publicPath)) throw new Error('Refusing to overwrite existing signing keys; use a new directory');

writeFileSync(privatePath, `ACTIVATION_SIGNING_KEY_ID=${keyId}\nACTIVATION_SIGNING_PRIVATE_KEY=${privateDer.toString('base64')}\n`, { mode: 0o600, flag: 'wx' });
writeFileSync(publicPath, `WK_ACTIVATION_PUBLIC_KEY=${publicDer.toString('base64')}\nWK_ACTIVATION_TRUSTED_PUBLIC_KEYS='${JSON.stringify({ [keyId]: publicDer.toString('base64') })}'\n`, { mode: 0o644, flag: 'wx' });
try { chmodSync(privatePath, 0o600); } catch {}

console.log(`Private signing environment written to: ${privatePath}`);
console.log(`Gateway public-key environment written to: ${publicPath}`);
console.log('Keep the private file only on the management platform; never copy it to a gateway.');
