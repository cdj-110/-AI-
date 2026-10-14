const assert = require('node:assert/strict')
const { chromium } = require(process.argv[2] || 'playwright')
const base = process.argv[3] || 'http://127.0.0.1:3029'
async function main() {
  const browser = await chromium.launch({ channel: 'chrome', headless: true })
  try {
    const context = await browser.newContext({ viewport: { width: 1600, height: 1000 } }), page = await context.newPage(), errors = []
    page.on('pageerror', e => errors.push(e.message))
    await context.addInitScript(() => {
      sessionStorage.setItem('wk_Token', JSON.stringify({ token: 'synthetic' }))
      sessionStorage.setItem('wk_UserId', 'menu-user'); sessionStorage.setItem('wk_Config_id', 'menu-test')
      sessionStorage.setItem('wk_ApiBaseUrl', 'http://127.0.0.1:5174/api'); sessionStorage.setItem('wk_PlatformOrigin', 'http://127.0.0.1:5174')
    })
    const line = { id: 'line', key: 'Figure', isGroup: false, attr: { x: 600, y: 300, w: 300, h: 5, contentW: 300, contentH: 5, zIndex: -1 }, chartConfig: { key: 'Figure', chartKey: 'VFigure', conKey: 'VCFigure', title: '线-01', category: 'Graph', categoryName: '基本图形', package: 'BasicShapes', chartFrame: 'static', image: 'Figure.png' }, option: { borderColor: '#3774B6ff', chuxi: 5 }, interactActions: [] }
    let saves = 0
    await context.route('**/api/**', route => {
      const url = route.request().url()
      let body = { main_data: { item: [{ title: '画布1', componentList: [line] }] } }
      if (url.includes('/auth/activity')) body = { data: { accessToken: 'synthetic-fresh', idleTimeoutSeconds: 3600 } }
      if (url.includes('/point-bindings')) body = { code: 200, msg: 'ok' }
      if (url.includes('/conf/config') && route.request().method() === 'POST') { saves++; body = { success: true, message: '模拟保存成功' } }
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) })
    })
    await context.route('http://127.0.0.1:5174/cloud-scada', route => route.fulfill({ status: 200, contentType: 'text/html', body: '<h1>模拟项目列表</h1>' }))
    const openEditor = async () => {
      await page.goto(`${base}/#/chart/home/menu-test`)
      await page.locator('[data-id="line"] .go-Figure').waitFor({ state: 'visible' }); await page.waitForTimeout(700)
    }
    await openEditor()
    const close = page.getByRole('button', { name: '返回云组态项目', exact: true })
    assert.equal((await close.textContent()).trim(), '', '顶部返回入口只能显示关闭图标')
    assert.equal(await page.locator('.go-header .go-system-setting').count(), 0)
    assert.equal(await page.getByText('奔跑的面条', { exact: true }).count(), 0)
    assert.equal(await page.getByText('退出登录', { exact: true }).count(), 0)
    const tools = page.locator('.go-chart-edit-tools')
    await tools.hover(); await page.waitForTimeout(600)
    const settings = tools.getByRole('button', { name: '编辑器设置', exact: true }), about = tools.getByRole('button', { name: '关于云组态', exact: true })
    const settingBox = await settings.boundingBox(), aboutBox = await about.boundingBox()
    assert.ok(aboutBox.y > settingBox.y, '关于云组态应位于设置下方')
    await settings.click()
    await page.locator('.go-system-setting').waitFor({ state: 'visible' })
    assert.equal(await page.locator('.go-system-setting h3').textContent(), '编辑器设置')
    await page.keyboard.press('Escape')
    await page.locator('.go-system-setting').waitFor({ state: 'hidden' })
    assert.equal(await page.getByRole('button', { name: '帮助', exact: true }).count(), 0)
    await tools.hover(); await page.waitForTimeout(600)
    await about.click()
    await page.locator('.go-system-info').waitFor({ state: 'visible' })
    assert.ok((await page.locator('.go-system-info').textContent()).includes('GoView'))
    await page.keyboard.press('Escape'); await page.locator('.go-system-info').waitFor({ state: 'hidden' })
    await page.getByRole('button', { name: '返回云组态项目', exact: true }).click()
    await page.waitForURL('http://127.0.0.1:5174/cloud-scada')
    await openEditor()
    const change = async () => {
      await page.locator('[data-id="line"] .go-Figure').click()
      const r = await page.locator('[data-id="line"] [data-endpoint="end"]').boundingBox()
      await page.mouse.move(r.x + r.width / 2, r.y + r.height / 2); await page.mouse.down()
      await page.mouse.move(r.x + r.width / 2 + 30, r.y + r.height / 2, { steps: 4 }); await page.mouse.up()
    }
    await change()
    await page.getByRole('button', { name: '返回云组态项目', exact: true }).click()
    await page.getByText('有未保存的修改', { exact: true }).waitFor({ state: 'visible' })
    await page.getByRole('button', { name: '继续编辑', exact: true }).click()
    assert.ok(page.url().includes('/chart/home/menu-test'))
    await page.keyboard.press('Control+s')
    await page.getByText('模拟保存成功', { exact: true }).waitFor({ state: 'visible', timeout: 15000 })
    assert.equal(saves, 1)
    await page.getByRole('button', { name: '返回云组态项目', exact: true }).click()
    await page.waitForURL('http://127.0.0.1:5174/cloud-scada')
    await openEditor(); await change()
    await page.getByRole('button', { name: '返回云组态项目', exact: true }).click()
    await page.getByRole('button', { name: '仍然返回', exact: true }).click()
    await page.waitForURL('http://127.0.0.1:5174/cloud-scada')
    assert.equal(saves, 1, '返回不可自动保存或退出登录')
    assert.deepEqual(errors, [])
    console.log('PASS: icon-only close/return, sidebar settings with about below, preserved attribution, clean/dirty/saved/discard return; no page errors')
  } finally { await browser.close() }
}
main().catch(e => { console.error(e); process.exitCode = 1 })
