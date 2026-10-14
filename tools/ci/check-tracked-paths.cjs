'use strict';
const { spawnSync } = require('node:child_process');

// Deliberately limited path policy; this is not a content-based secret scanner.
function inspectGitResult(result) {
  if (result.error || result.status !== 0 || result.signal) {
    throw new Error('Cannot enumerate tracked paths; refusing to continue.');
  }
  if (typeof result.stdout !== 'string') throw new Error('Missing tracked path output.');
  const paths = result.stdout.split('\0').filter(Boolean);
  const forbidden = /(^|\/)(\.env|\.env\.local|\.env\.production\.local)$|(^|\/)secrets\//;
  if (paths.some(p => forbidden.test(p))) {
    throw new Error('Tracked local environment or secrets path detected; refusing to continue.');
  }
  return paths.length;
}

if (require.main === module) {
  try {
    const result = spawnSync('git', ['ls-files', '-z'], {
      encoding: 'utf8', maxBuffer: 64 * 1024 * 1024, timeout: 30000,
    });
    const count = inspectGitResult(result);
    console.log(`Tracked-path policy passed (${count} paths).`);
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
module.exports = { inspectGitResult };
