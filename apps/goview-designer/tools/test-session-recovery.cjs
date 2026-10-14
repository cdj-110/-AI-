const fs = require('node:fs'), path = require('node:path'), vm = require('node:vm'), assert = require('node:assert/strict')
const ts = require('typescript'), { parse } = require('@vue/compiler-sfc')
const root = path.resolve(__dirname, '..')
function compile(source) { return ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText }
const claimsToken = 'header.' + Buffer.from(JSON.stringify({ sub: 'original-user', tenantId: 'tenant-a' })).toString('base64url') + '.signature'

async function recoveryFlow() {
  const stored = new Map([['wk_UserId', 'original-user'], ['wk_Config_id', 'project-a'], ['wk_ApiBaseUrl', 'http://localhost:5174/api']])
  let renews = 0, drafts = 0, profile = { id: 'original-user', tenantId: 'tenant-a' }, token = claimsToken
  const events = [], persisted = []
  const context = { exports: {}, URL, atob, Event,
    sessionStorage: { getItem: key => stored.get(key) ?? null },
    window: { location: { origin: 'http://localhost:8080' }, dispatchEvent: event => events.push(event.type) },
    fetch: async () => ({ ok: true, json: async () => ({ data: profile }) }),
    require: name => name === 'vue' ? { reactive: value => value } : {
      readPlatformToken: () => token,
      refreshPlatformSession: async () => { renews++; throw new Error('expired') },
      persistPlatformAuth: value => { persisted.push(value); token = value.token }
    }
  }
  vm.runInNewContext(compile(fs.readFileSync(path.join(root, 'src/utils/sessionRecovery.ts'), 'utf8')), context)
  const api = context.exports
  api.setRecoveryDraftSaver(() => { drafts++; return true })
  const first = api.recoverPlatformSession(), second = api.recoverPlatformSession()
  assert.equal(first, second)
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(renews, 1); assert.equal(drafts, 1)
  assert.equal(api.sessionRecovery.paused, true); assert.equal(api.sessionRecovery.visible, true)
  assert.match(api.recoveryDraftKey(), /tenant-a:original-user:project-a$/)
  profile = { id: 'different-user', tenantId: 'tenant-a' }
  await assert.rejects(api.acceptRecoveredSession('foreign'), /原账号/)
  profile = { id: 'original-user', tenantId: 'different-tenant' }
  await assert.rejects(api.acceptRecoveredSession('foreign'), /原账号/)
  assert.equal(persisted.length, 0); assert.equal(api.sessionRecovery.paused, true)
  profile = { id: 'original-user', tenantId: 'tenant-a' }
  await api.acceptRecoveredSession('fresh-token')
  assert.equal(await first, 'fresh-token'); assert.equal(await second, 'fresh-token')
  assert.equal(api.sessionRecovery.paused, false); assert.equal(api.sessionRecovery.visible, false)
  assert.equal(persisted.length, 1)
  assert.deepEqual(events, ['wk-session-paused', 'wk-session-restored'])
}

async function modalMessageGuards() {
  const descriptor = parse(fs.readFileSync(path.join(root, 'src/components/SessionRecovery.vue'), 'utf8')).descriptor
  const source = descriptor.scriptSetup.content + '\nexports.testing = { receive, frame };'
  let accepted = 0
  const state = { paused: true, visible: true, draftSaved: true, error: '' }
  const ctx = { exports: {}, crypto: { randomUUID: () => 'one-time-nonce' },
    window: { addEventListener() {}, removeEventListener() {} },
    require: name => {
      if (name === 'vue') return { ref: value => ({ value }), watch: (read, cb) => cb(read()), onBeforeUnmount() {} }
      if (name.includes('chartEditStore')) return { useChartEditStore: () => ({}) }
      if (name === '@/utils') return { JSONStringify: JSON.stringify }
      return { sessionRecovery: state, recoveryIdentity: () => ({ userId: 'user', tenantId: 'tenant' }),
        recoveryPlatformOrigin: () => 'http://localhost:5174', setRecoveryDraftSaver: () => () => {},
        acceptRecoveredSession: async () => { accepted++ } }
    }
  }
  vm.runInNewContext(compile(source), ctx)
  const { receive, frame } = ctx.exports.testing
  const child = { postMessage() {} }; frame.value = { contentWindow: child }
  const valid = { origin: 'http://localhost:5174', source: child, data: { type: 'wk-recovery-auth', nonce: 'one-time-nonce', token: 'fresh' } }
  await receive({ ...valid, origin: 'http://attacker.invalid' })
  await receive({ ...valid, source: {} })
  await receive({ ...valid, data: { ...valid.data, nonce: 'previous-nonce' } })
  assert.equal(accepted, 0)
  await receive(valid); assert.equal(accepted, 1)
}

async function loginBridgeGuards() {
  const file = path.resolve(root, '../web/src/utils/session-login.ts')
  const source = fs.readFileSync(file, 'utf8').replaceAll('import.meta.env', 'testEnv')
  let initialized = 0, receive
  const posts = [], parent = { postMessage: (...args) => posts.push(args) }
  const ctx = { exports: {}, URL, testEnv: { DEV: false }, location: { protocol: 'http:', hostname: 'localhost', origin: 'http://localhost:5174' },
    window: { parent, addEventListener: (name, cb) => { receive = cb }, removeEventListener() {} },
    require: () => ({ getAuthItem: () => null }) }
  vm.runInNewContext(compile(source), ctx)
  const bridge = ctx.exports.initSessionLogin(() => initialized++)
  const valid = { source: parent, origin: 'http://localhost:8080', data: { type: 'wk-recovery-init', nonce: 'nonce', userId: 'user', tenantId: 'tenant' } }
  await receive({ ...valid, origin: 'http://localhost:9999' }); await receive({ ...valid, source: {} })
  assert.equal(initialized, 0); bridge.send('fresh'); assert.equal(posts.length, 1)
  await receive(valid); assert.equal(initialized, 1)
  await receive(valid); assert.equal(initialized, 1)
  bridge.send('fresh'); assert.equal(posts.length, 2); assert.equal(posts[1][1], 'http://localhost:8080')
  assert.equal(posts[1][0].nonce, 'nonce')
  assert.throws(() => ctx.exports.assertRecoveryIdentity({ id: 'other', tenantId: 'tenant' }, { userId: 'user', tenantId: 'tenant' }), /原账号/)
}

async function requestRecoveryRules() {
  const file = path.join(root, 'src/utils/request.ts')
  const source = fs.readFileSync(file, 'utf8').replaceAll('import.meta.env', 'testEnv')
  let requestHandler, responseHandler, errorHandler, retries = 0, recoveries = 0
  const state = { paused: false }
  const instance = { request: async config => { retries++; return config },
    interceptors: { request: { use: cb => { requestHandler = cb } }, response: { use: (ok, bad) => { responseHandler = ok; errorHandler = bad } } }
  }
  class Cancel extends Error {}
  const axios = { create: () => instance, Cancel, CancelToken: function() {}, isCancel: error => error instanceof Cancel }
  const context = { exports: {}, testEnv: { PROD: true }, sessionStorage: { getItem: () => 'http://localhost:5174/api' },
    window: { location: { host: 'localhost' }, $message: { warning() {} } },
    require: name => {
      if (name === 'axios') return { default: axios }
      if (name.includes('sessionRecovery')) return { sessionRecovery: state, recoverPlatformSession: async () => { recoveries++; return 'fresh' } }
      if (name.includes('platformAuth')) return { readPlatformToken: () => 'old' }
      return { requestErrorMessage: () => 'failed' }
    }
  }
  vm.runInNewContext(compile(source), context)
  const error = method => ({ response: { status: 401 }, config: { method, headers: {}, url: '/conf/config' } })
  const result = await errorHandler(error('get'))
  assert.equal(result.headers.Authorization, 'Bearer fresh'); assert.equal(retries, 1)
  await assert.rejects(errorHandler(error('post')), /重新点击保存/)
  assert.equal(retries, 1)
  const second401 = error('get'); second401.config.__wkRetried = true
  await assert.rejects(errorHandler(second401), /仍无法通过认证/)
  assert.equal(recoveries, 2)
  state.paused = true
  await assert.rejects(requestHandler({}), /数据请求已暂停/)
  state.paused = false
  await responseHandler({ data: { code: 999999999 }, config: { method: 'get', headers: {} } })
  assert.equal(retries, 2)
}

Promise.resolve().then(recoveryFlow).then(modalMessageGuards).then(loginBridgeGuards).then(requestRecoveryRules)
  .then(() => console.log('PASS: single-flight recovery, scoped draft, server identity validation, origin/source/nonce guards, paused requests, bounded GET retry, no write replay'))
  .catch(error => { console.error(error); process.exitCode = 1 })
