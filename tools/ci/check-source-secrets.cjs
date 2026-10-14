const {execFileSync} = require('node:child_process');
const fs = require('node:fs');
const paths = execFileSync('git', ['ls-files','-z'], {encoding:'utf8',maxBuffer:64*1024*1024}).split('\0').filter(Boolean);
const strong = /-----BEGIN (?:RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----|\bgh[pousr]_[A-Za-z0-9]{30,}\b|\bgithub_pat_[A-Za-z0-9_]{30,}\b|\b(?:AKIA|ASIA)[A-Z0-9]{16}\b/;
const failures=[];
for(const file of paths) {
  if(/(^|\/)(node_modules|secrets|credentials|\.git)\//.test(file) || /(^|\/)\.env(?:\..*)?$/.test(file) && !/\.(example|sample)$/.test(file) || /\.(pem|key|pfx|p12)$/.test(file)) failures.push(file+': forbidden path');
  const data=fs.readFileSync(file);
  if(strong.test(data.toString('utf8'))) failures.push(file+': credential pattern');
}
if(failures.length){console.error(failures.join('\n'));process.exit(1);}
console.log(`Source credential/path checks passed for ${paths.length} files; pattern checks are not a complete secret audit.`);
