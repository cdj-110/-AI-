const fs = require('node:fs');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const cwd = path.resolve(__dirname, '../../apps/web');
const sentinel = path.join(cwd, 'src/__ci_typecheck_negative_control.ts');
if (fs.existsSync(sentinel)) throw new Error('Refusing to overwrite existing sentinel');
try {
  fs.writeFileSync(sentinel, 'export const ciSentinel: string = 123;\n');
  for (const mode of ['tenant', 'admin']) {
    const result = spawnSync('npm', ['run', `build:${mode}`], {cwd, encoding:'utf8', timeout:180000, maxBuffer:8*1024*1024});
    const output = (result.stdout || '') + (result.stderr || '');
    if (result.error || result.signal || result.status === 0 || !output.includes('__ci_typecheck_negative_control.ts') || !output.includes('TS2322') || output.includes('modules transformed')) {
      console.error(output); throw new Error(`Build ${mode} did not reject the deliberate type error before bundling`);
    }
    console.log(`${mode}: deliberate TS2322 rejected before bundling`);
  }
} finally { fs.unlinkSync(sentinel); }
