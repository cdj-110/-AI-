// 在独立无头浏览器中验证已部署制品；接口全部模拟，不使用真实登录信息或修改业务数据。
const assert = require('node:assert/strict')
const { chromium } = require(process.argv[2] || 'playwright')
async function main() {
  const browser = await chromium.launch({ channel: 'chrome', headless: true })
  try {
    const context = await browser.newContext()
    const errors = []
    context.on('page', page => page.on('pageerror', error => errors.push(error.message)))
    await context.route('**/api/**', route => route.fulfill({
      status: 401, contentType: 'application/json', body: JSON.stringify({ code: 401, message: '模拟登录过期' })
    }))
    await context.addInitScript(() => {
      if (location.port !== '8080') return
      const claims = btoa(JSON.stringify({ sub: 'smoke-user', tenantId: 'smoke-tenant' }))
      sessionStorage.setItem('wk_Token', JSON.stringify({ token: `smoke.${claims}.signature` }))
      sessionStorage.setItem('wk_UserId', 'smoke-user')
      sessionStorage.setItem('wk_Config_id', 'smoke-project')
      sessionStorage.setItem('wk_TenantId', 'smoke-tenant')
      sessionStorage.setItem('wk_ApiBaseUrl', 'http://127.0.0.1:5174/api')
      sessionStorage.setItem('wk_PlatformOrigin', 'http://127.0.0.1:5174')
    })
    const platform = await context.newPage()
    await platform.goto('http://127.0.0.1:5174/session-login', { waitUntil: 'domcontentloaded' })
    const [page] = await Promise.all([
      platform.waitForEvent('popup'),
      platform.evaluate(() => {
        let child
        window.addEventListener('message', event => {
          if (event.source !== child || event.origin !== 'http://127.0.0.1:8080' || event.data?.type !== 'wk-auth-ready' || event.data.nonce !== 'smoke-once') return
          child.postMessage({ type: 'wk-platform-auth', nonce: 'smoke-once',
            token: `smoke.${btoa(JSON.stringify({ sub: 'smoke-user', tenantId: 'smoke-tenant' }))}.signature`,
            user_id: 'smoke-user', tenant_id: 'smoke-tenant', config_id: 'smoke-project',
            api_base: 'http://127.0.0.1:5174/api', platform_origin: 'http://127.0.0.1:5174'
          }, event.origin)
        })
        child = window.open('http://127.0.0.1:8080/#/chart/home/smoke-project?auth_bridge=1&auth_nonce=smoke-once&platform_origin=http%3A%2F%2F127.0.0.1%3A5174')
      })
    ])
    await page.locator('#go-chart-edit-layout').waitFor({ state: 'visible', timeout: 30000 })
    await page.locator('.session-recovery-mask').waitFor({ state: 'visible', timeout: 30000 })
    const card = page.frameLocator('iframe[title="平台登录"]').locator('.auth-card')
    await card.waitFor({ state: 'visible', timeout: 30000 })
    assert.match(await card.innerText(), /租户登录/)
    assert.ok(!page.url().includes('auth_nonce'), '首次握手后必须清理桥接参数')
    await page.reload({ waitUntil: 'domcontentloaded' })
    await page.locator('#go-chart-edit-layout').waitFor({ state: 'visible', timeout: 10000 })
    await page.frameLocator('iframe[title="平台登录"]').locator('.auth-card').waitFor({ state: 'visible', timeout: 10000 })
    assert.equal(errors.length, 0, `浏览器启动异常：${errors.join('; ')}`)
    console.log('PASS: platform popup handshake, editor startup, bridge parameter cleanup, embedded expired-session login, reload, no browser page errors')
  } finally { await browser.close() }
}
main().catch(error => { console.error(error.message); process.exitCode = 1 })
