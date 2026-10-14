// Generate into an isolated build directory without replacing DLLs used by
// the running local ingest service. This does not migrate a database.
const { mkdirSync, mkdtempSync, readFileSync, writeFileSync } = require('node:fs');
const { resolve, join } = require('node:path');
const { execFileSync } = require('node:child_process');
const root=resolve(__dirname,'..');
const cache=resolve(root,'.cache');
mkdirSync(cache,{recursive:true});
const directory=mkdtempSync(join(cache,'ingest-license-client-'));
const output=join(directory,'client').replace(/\\/g,'/');
const schema=readFileSync(join(root,'prisma/schema.prisma'),'utf8').replace(/generator client\s*\{/,`generator client {\n  output = ${JSON.stringify(output)}`);
const schemaPath=join(directory,'schema.prisma');
writeFileSync(schemaPath,schema);
execFileSync(process.execPath,[require.resolve('prisma/build/index.js'),'generate','--schema',schemaPath],{cwd:root,stdio:'inherit',env:{...process.env,PRISMA_GENERATE_SKIP_AUTOINSTALL:'true'}});
const generated=readFileSync(join(directory,'client/index.d.ts'),'utf8');
if(!generated.includes('licenseSequence') || !generated.includes('activationSecretEncrypted')) throw new Error('Authorization fields missing from ingest client');
console.log('Isolated ingest Prisma client verified; running service unchanged.');
