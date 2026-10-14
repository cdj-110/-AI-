const fs = require('fs'), path = require('path'), ts = require('typescript'), vm = require('vm'), assert = require('node:assert/strict')
const source = fs.readFileSync(path.resolve(__dirname, '../src/utils/messageListener.ts'), 'utf8')
const js = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText
function setup({ reload = false, cached = false, storedProject = 'project-a', bridge = true } = {}) {
  const timers = new Map(), listeners = new Map(), posts = [], cleanups = [], persisted = []
  const opener = { postMessage: (...args) => posts.push(args) }
  const context = {
    exports: {}, performance: { getEntriesByType: () => [{ type: reload ? 'reload' : 'navigate' }] },
    sessionStorage: { getItem: key => ({ wk_Config_id: cached ? storedProject : null, wk_ApiBaseUrl: cached ? 'http://localhost:3100/api' : null })[key] },
    window: {
      location: { hostname: 'localhost', hash: '#/chart/home/project-a' + (bridge ? '?auth_bridge=1&auth_nonce=once&platform_origin=http%3A%2F%2Flocalhost%3A5174' : '') },
      opener, setTimeout: fn => { const id = timers.size + 1; timers.set(id, fn); return id },
      clearTimeout: id => timers.delete(id), addEventListener: (type, fn) => listeners.set(type, fn),
      removeEventListener: (type, fn) => { if (listeners.get(type) === fn) listeners.delete(type) }
    },
    URL, URLSearchParams,
    require: () => ({ readPlatformToken: () => cached ? 'test-session' : '', persistPlatformAuth: value => persisted.push(value),
      removePlatformAuthParamsFromAddress: () => { cleanups.push(true); context.window.location.hash = '#/chart/home/project-a' } })
  }
  vm.runInNewContext(js, context)
  return { context, timers, listeners, posts, cleanups, persisted, opener }
}
async function main() {
  const reload = setup({ reload: true, cached: true })
  await reload.context.exports.initMessageListener()
  assert.equal(reload.timers.size, 0); assert.equal(reload.posts.length, 0); assert.equal(reload.cleanups.length, 1)
  const clean = setup({ reload: true, cached: true, bridge: false })
  await clean.context.exports.initMessageListener(); assert.equal(clean.timers.size, 0)
  for (const options of [{ cached: true }, { reload: true }, { reload: true, cached: true, storedProject: 'other-project' }]) {
    const first = setup(options), promise = first.context.exports.initMessageListener()
    assert.equal(first.timers.size, 1); assert.equal(first.posts.length, 1)
    const receive = first.listeners.get('message')
    receive({ origin: 'http://attacker.invalid', source: first.opener, data: { type: 'wk-platform-auth', nonce: 'once' } })
    receive({ origin: 'http://localhost:5174', source: {}, data: { type: 'wk-platform-auth', nonce: 'once' } })
    receive({ origin: 'http://localhost:5174', source: first.opener, data: { type: 'wk-platform-auth', nonce: 'wrong' } })
    assert.equal(first.persisted.length, 0); assert.equal(first.timers.size, 1)
    receive({ origin: 'http://localhost:5174', source: first.opener, data: { type: 'wk-platform-auth', nonce: 'once' } })
    await promise
    assert.equal(first.persisted.length, 1); assert.equal(first.timers.size, 0); assert.equal(first.listeners.size, 0)
  }
  const timeout = setup(), promise = timeout.context.exports.initMessageListener()
  timeout.timers.values().next().value(); await promise; assert.equal(timeout.listeners.size, 0)
  const entry = fs.readFileSync(path.resolve(__dirname, '../src/main.ts'), 'utf8')
  assert.match(entry, /await router.isReady\(\)[\s\S]*?removePlatformAuthParamsFromAddress\(\)/)
  console.log('PASS: reload restores same-project session without 15-second bridge wait; first navigation, cross-project handshake, origin/source/nonce checks and cleanup preserved')
}
main().catch(error => { console.error(error); process.exitCode = 1 })
