const fs = require('fs'), path = require('path'), ts = require('typescript'), assert = require('node:assert/strict')
const source = fs.readFileSync(path.resolve(__dirname, '../src/utils/lazyComponentRegistry.ts'), 'utf8')
const result = { exports: {} }
new Function('exports', 'module', ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText)(result.exports, result)
let loaded = 0
const modules = {
  './components/BasicShapes/Mores/Number/index.vue': async () => { loaded++; return { default: 'basic-number' } },
  './components/Decorates/Mores/Number/index.vue': async () => { loaded++; return { default: 'decorate-number' } },
  './components/BasicShapes/Mores/Number/config.vue': async () => { loaded++; return { default: 'number-settings' } }
}
const registry = result.exports.createLazyComponentRegistry(modules, loader => ({ loader }))
const descriptor = { package: 'BasicShapes', category: 'Mores', key: 'Number' }
const view = registry(descriptor, 'index.vue')
assert.equal(loaded, 0, 'registration must not download components')
assert.equal(view, registry(descriptor, 'index.vue'), 'stable wrapper identity')
assert.notEqual(view, registry(descriptor, 'config.vue'))
assert.notEqual(view, registry({ ...descriptor, package: 'Decorates' }, 'index.vue'))
assert.equal(registry({ key: 'Missing' }, 'index.vue'), undefined)
assert.equal(view, registry({ key: 'ImageAlias', redirectComponent: 'BasicShapes/Mores/Number' }, 'index.vue'))
assert.equal(view, registry({ key: 'Number' }, 'index.vue'))
Promise.all([view.loader(), registry(descriptor, 'config.vue').loader()]).then(values => {
  assert.deepEqual(values.map(value => value.default), ['basic-number', 'number-settings'])
  assert.equal(loaded, 2)
  const nginx = fs.readFileSync(path.resolve(__dirname, '../nginx.conf'), 'utf8')
  assert.match(nginx, /gzip_static on/)
  assert.match(nginx, /immutable/)
  assert.match(nginx, /location \/monacoeditorwork\/ \{\s*add_header Cache-Control "no-cache"/)
  console.log('PASS: lazy component loading, stable registry, duplicate names, redirects, separate config panels, compression/cache configuration')
}).catch(error => { console.error(error); process.exitCode = 1 })
