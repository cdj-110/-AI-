const assert = require('node:assert/strict'), fs = require('node:fs'), path = require('node:path')
const { chromium } = require(process.argv[2] || 'playwright')
const deployed = process.argv.includes('--deployed')
const output = path.resolve(__dirname, `../../../outputs/login-background-20261013${deployed ? '-deployed' : ''}`)
const reference = path.resolve(__dirname, '../../../outputs/login-background-20261013/source-gateway.png')
async function main() {
  fs.mkdirSync(output, { recursive: true })
  const browser = await chromium.launch({ channel: 'chrome', headless: true })
  try {
    const backgrounds = []
    for (const [mode, port, title] of [['tenant', deployed ? 5174 : 5434, '租户登录'], ['admin', deployed ? 5176 : 5436, '公司管理登录']]) {
      const context = await browser.newContext({ viewport: { width: 1920, height: 911 }, deviceScaleFactor: 1 })
      const page = await context.newPage(), errors = [], writes = []
      page.on('pageerror', error => errors.push(error.message))
      await context.route('**/api/**', route => { if (route.request().method() !== 'GET') writes.push(route.request().url()); return route.fulfill({ status: 200, contentType: 'application/json', body: '{"code":0,"data":{}}' }) })
      await page.goto(`http://127.0.0.1:${port}/login`)
      await page.getByRole('heading', { name: title }).waitFor({ state: 'visible' })
      const css = await page.locator('.auth-page').evaluate(e => getComputedStyle(e).backgroundImage)
      assert.ok(css.includes('radial-gradient(circle at 20% 10%'))
      assert.ok(css.includes('rgb(219, 234, 254)') && css.includes('32%') && css.includes('135deg'))
      backgrounds.push(css)
      await page.screenshot({ path: path.join(output, `${mode}-desktop.png`) })
      await page.getByRole('button', { name: '登录', exact: true }).click()
      await page.waitForFunction(() => document.querySelectorAll('.el-form-item__error').length >= 2)
      await page.getByText('验证码登录', { exact: true }).click()
      await page.getByPlaceholder('请输入 6 位验证码', { exact: true }).waitFor({ state: 'visible' })
      await page.setViewportSize({ width: 375, height: 812 })
      await page.goto(`http://127.0.0.1:${port}/login`)
      await page.getByRole('heading', { name: title }).waitFor({ state: 'visible' })
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth), false)
      await page.screenshot({ path: path.join(output, `${mode}-mobile.png`), fullPage: true })
      await page.goto(`http://127.0.0.1:${port}/session-login`)
      await page.locator('.embedded-login').waitFor({ state: 'visible' })
      const embedded = await page.locator('.embedded-login').evaluate(e => ({ image: getComputedStyle(e).backgroundImage, color: getComputedStyle(e).backgroundColor }))
      assert.equal(embedded.image, 'none'); assert.equal(embedded.color, 'rgb(255, 255, 255)')
      assert.deepEqual(writes, []); assert.deepEqual(errors, [])
      await context.close()
    }
    assert.equal(backgrounds[0], backgrounds[1])
    const compare = await browser.newPage({ viewport: { width: 3840, height: 911 }, deviceScaleFactor: 1 })
    const data = file => `data:image/png;base64,${fs.readFileSync(file).toString('base64')}`
    for (const mode of ['tenant', 'admin']) {
      await compare.setContent(`<html><body style="margin:0;display:flex"><img width="1920" height="911" src="${data(reference)}"><img width="1920" height="911" src="${data(path.join(output, `${mode}-desktop.png`))}"></body></html>`)
      await compare.evaluate(() => Promise.all([...document.images].map(img => img.decode())))
      await compare.screenshot({ path: path.join(output, `${mode}-comparison.png`) })
      await compare.setViewportSize({ width: 1200, height: 360 })
      await compare.setContent(`<html><body style="margin:0;display:flex"><div style="width:600px;height:360px;overflow:hidden"><img src="${data(reference)}" style="width:1920px;height:911px;max-width:none"></div><div style="width:600px;height:360px;overflow:hidden"><img src="${data(path.join(output, `${mode}-desktop.png`))}" style="width:1920px;height:911px;max-width:none"></div></body></html>`)
      await compare.evaluate(() => Promise.all([...document.images].map(img => img.decode())))
      await compare.screenshot({ path: path.join(output, `${mode}-background-detail.png`) })
      await compare.setViewportSize({ width: 3840, height: 911 })
    }
    console.log('PASS: matching admin/tenant gateway backgrounds, validation and mode switching, mobile no horizontal overflow, embedded login remains white; no real API writes or page errors; screenshots:', output)
  } finally { await browser.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
