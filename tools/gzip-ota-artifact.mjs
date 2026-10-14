import { createReadStream, createWriteStream } from 'node:fs';
import { createGzip } from 'node:zlib';
import { pipeline } from 'node:stream/promises';

const [source, output] = process.argv.slice(2);
if (!source || !output) {
  throw new Error('用法：node tools/gzip-ota-artifact.mjs <gateway-binary> <gateway.gz>');
}

await pipeline(
  createReadStream(source),
  createGzip({ level: 9, mtime: 0 }),
  createWriteStream(output, { flags: 'wx', mode: 0o600 }),
);

console.log(output);
