import { randomBytes } from 'crypto';
import { mkdirSync, readFileSync, renameSync, writeFileSync } from 'fs';
import { dirname } from 'path';

const insecureSecrets = new Set(['', 'weikong_iot_secret']);

function isValidSecret(secret: string) {
  return secret.length >= 32 && !insecureSecrets.has(secret);
}

export function ensureJwtSecret() {
  const configured = (process.env.JWT_SECRET ?? '').trim();
  if (isValidSecret(configured)) return configured;

  const secretFile = (process.env.JWT_SECRET_FILE ?? '').trim();
  if (secretFile) {
    try {
      const persisted = readFileSync(secretFile, 'utf8').trim();
      if (isValidSecret(persisted)) {
        process.env.JWT_SECRET = persisted;
        return persisted;
      }
    } catch (error) {
      const code = (error as NodeJS.ErrnoException).code;
      if (code !== 'ENOENT') throw error;
    }

    mkdirSync(dirname(secretFile), { recursive: true, mode: 0o700 });
    const generated = randomBytes(48).toString('base64url');
    const temporaryFile = `${secretFile}.${process.pid}.tmp`;
    writeFileSync(temporaryFile, `${generated}\n`, { encoding: 'utf8', mode: 0o600, flag: 'wx' });
    renameSync(temporaryFile, secretFile);
    process.env.JWT_SECRET = generated;
    return generated;
  }

  if (process.env.NODE_ENV === 'production') {
    throw new Error('JWT_SECRET or JWT_SECRET_FILE must provide a random value of at least 32 characters in production');
  }

  process.env.JWT_SECRET = 'development-only-weikong-jwt-secret-change-me';
  return process.env.JWT_SECRET;
}
