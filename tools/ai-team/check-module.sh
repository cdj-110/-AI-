#!/bin/sh
set -eu
profile="${1:?profile required}"
case "$profile" in web|api|ingest) ;; *) exit 2 ;; esac
cp -R "/workspace/apps/$profile/." /check/
cd /check
node - "$profile" <<'JS'
const fs=require('node:fs');
const assert=require('node:assert/strict');
for(const name of ['package.json','package-lock.json']) {
  assert.deepStrictEqual(JSON.parse(fs.readFileSync(name,'utf8')),JSON.parse(fs.readFileSync(`/deps/${process.argv[2]}/${name}`,'utf8')),`Dependency manifest mismatch: ${name}`);
}
console.log('Dependency manifests match pinned image (line endings normalized by JSON parsing).');
JS
ln -s "/deps/$profile/node_modules" node_modules
export DATABASE_URL=postgresql://ci:ci@127.0.0.1:1/ci_unreachable
export NODE_OPTIONS=--max-old-space-size=3072
case "$profile" in
  web) npm run typecheck; npm run build:all ;;
  api) npm run prisma:generate; npm run build; node scripts/verify-tenant-user-action-permissions.cjs ;;
  ingest) npm run prisma:generate; npm run build; node scripts/verify-startup-readiness.cjs; node scripts/verify-gateway-point-access.cjs ;;
esac
