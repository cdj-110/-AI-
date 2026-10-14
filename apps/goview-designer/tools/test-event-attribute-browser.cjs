const assert = require('node:assert/strict')
const { chromium } = require(process.argv[2] || 'playwright')
const base = process.argv[3] || 'http://127.0.0.1:3029'
async function main() {
  const browser = await chromium.launch({ channel: 'chrome', headless: true })
  try {
    const context = await browser.newContext({ viewport: { width: 1600, height: 1000 } })
    const page = await context.newPage(), errors = []
    page.on('pageerror', error => errors.push(error.message))
    await context.addInitScript(() => {
      sessionStorage.setItem('wk_Token', JSON.stringify({ token: 'synthetic' }))
      sessionStorage.setItem('wk_UserId', 'preview-user')
      sessionStorage.setItem('wk_Config_id', 'event-preview')
      sessionStorage.setItem('wk_ApiBaseUrl', 'http://127.0.0.1:5174/api')
    })
    const event = index => ({ id: `event-${index}`, isNewly: true, form: {
      peizhiType: '表单配置', changeType: '单击', changeAttr: '更新属性', targetComponent: 'test-line', saveAttrArr: {}
    } })
    const line = { id: 'test-line', key: 'Figure', isGroup: false,
      attr: { x: 600, y: 300, w: 300, h: 5, contentW: 300, contentH: 5, zIndex: -1 },
      chartConfig: { key: 'Figure', chartKey: 'VFigure', conKey: 'VCFigure', title: '线-01', category: 'Graph', categoryName: '基本图形', package: 'BasicShapes', chartFrame: 'static', image: 'Figure.png' },
      option: { borderColor: '#3774B6ff', chuxi: 5 }, interactActions: [event(1), event(2)]
    }
    await context.route('**/api/**', route => {
      const url = route.request().url()
      const body = url.includes('/auth/activity')
        ? { data: { accessToken: 'synthetic-fresh', idleTimeoutSeconds: 3600 } }
        : { main_data: { item: [{ title: '画布1', componentList: [line] }] } }
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) })
    })
    await page.goto(`${base}/#/chart/home/event-preview`, { waitUntil: 'domcontentloaded' })
    const shape = page.locator('[data-id="test-line"]'), visual = shape.locator('.go-Figure')
    await visual.waitFor({ state: 'visible', timeout: 30000 })
    const layering = await page.evaluate(() => {
      const overlay = getComputedStyle(document.querySelector('.go-edit-align-line'))
      const componentZ = [...document.querySelectorAll('.go-shape-box')].map(element => Number(getComputedStyle(element).zIndex) || 0)
      return { overlayZ: Number(overlay.zIndex), maxComponentZ: Math.max(0, ...componentZ), position: overlay.position, isolation: overlay.isolation, pointerEvents: overlay.pointerEvents }
    })
    assert.ok(layering.overlayZ > layering.maxComponentZ)
    assert.equal(layering.position, 'absolute'); assert.equal(layering.isolation, 'isolate'); assert.equal(layering.pointerEvents, 'none')
    await shape.click()
    await page.getByText('事件', { exact: true }).click()
    await page.getByRole('button', { name: '编辑组件属性', exact: true }).first().click()
    const drawer = page.locator('.n-drawer')
    await drawer.waitFor({ state: 'visible' })
    assert.equal(await drawer.count(), 1, '多个事件只应打开一个属性面板')
    const thickness = drawer.getByPlaceholder('线条粗细')
    await thickness.fill('12'); await thickness.press('Enter')
    await page.waitForFunction(() => document.querySelector('[data-id="test-line"] .line-stroke')?.getAttribute('stroke-width') === '12')
    await drawer.getByRole('button', { name: '取消', exact: true }).click()
    await page.waitForFunction(() => document.querySelector('[data-id="test-line"] .line-stroke')?.getAttribute('stroke-width') === '5')
    await page.getByRole('button', { name: '编辑组件属性', exact: true }).first().click()
    await thickness.fill('18'); await thickness.press('Enter')
    await page.waitForFunction(() => document.querySelector('[data-id="test-line"] .line-stroke')?.getAttribute('stroke-width') === '18')
    await drawer.getByRole('button', { name: '保存', exact: true }).click()
    await page.waitForFunction(() => document.querySelector('[data-id="test-line"] .line-stroke')?.getAttribute('stroke-width') === '5')
    await page.getByRole('button', { name: '编辑组件属性', exact: true }).first().click()
    await page.waitForFunction(() => document.querySelector('[data-id="test-line"] .line-stroke')?.getAttribute('stroke-width') === '18')
    assert.equal(await thickness.inputValue(), '18')
    await drawer.getByRole('button', { name: '取消', exact: true }).click()
    await page.getByText('定制', { exact: true }).click()
    const angle = page.getByPlaceholder('角度', { exact: true })
    await angle.fill('90'); await angle.press('Enter')
    await page.waitForFunction(() => {
      const line = document.querySelector('[data-id="test-line"] .line-stroke')
      return Math.abs(Number(line?.getAttribute('x2')) - Number(line?.getAttribute('x1'))) < .01 && Number(line?.getAttribute('y2')) > Number(line?.getAttribute('y1')) + 100
    })
    const endpoint = shape.locator('[data-endpoint="end"]')
    const rect = await endpoint.boundingBox()
    await page.keyboard.down('Shift')
    await page.mouse.move(rect.x + rect.width / 2, rect.y + rect.height / 2); await page.mouse.down()
    await page.mouse.move(rect.x + rect.width / 2 + 110, rect.y + rect.height / 2 - 15, { steps: 6 }); await page.mouse.up()
    await page.keyboard.up('Shift')
    await page.waitForFunction(() => document.querySelector('input[placeholder="角度"]')?.value === '45')
    await page.keyboard.press('Control+z')
    await page.waitForFunction(() => document.querySelector('input[placeholder="角度"]')?.value === '90')
    await page.keyboard.press('Control+Shift+z')
    await page.waitForFunction(() => document.querySelector('input[placeholder="角度"]')?.value === '45')
    await page.getByPlaceholder('长度', { exact: true }).fill('450')
    await page.getByPlaceholder('长度', { exact: true }).press('Enter')
    assert.equal(await page.getByPlaceholder('长度', { exact: true }).inputValue(), '450')
    await page.getByText('实线', { exact: true }).click()
    await page.getByText('虚线', { exact: true }).click()
    assert.ok(await shape.locator('.line-stroke').getAttribute('stroke-dasharray'))
    await page.getByText('平头', { exact: true }).click()
    await page.getByText('圆头', { exact: true }).click()
    assert.equal(await shape.locator('.line-stroke').getAttribute('stroke-linecap'), 'round')
    await page.getByRole('checkbox', { name: '起点', exact: true }).click()
    await page.getByRole('checkbox', { name: '终点', exact: true }).click()
    assert.equal(await shape.locator('polygon').count(), 2)
    const strokeInput = page.getByPlaceholder('线条粗细')
    await strokeInput.fill('1'); await strokeInput.press('Enter')
    const hit = await page.evaluate(() => {
      const a = document.querySelector('[data-endpoint="start"]').getBoundingClientRect()
      const b = document.querySelector('[data-endpoint="end"]').getBoundingClientRect()
      const ax = a.left + a.width / 2, ay = a.top + a.height / 2, bx = b.left + b.width / 2, by = b.top + b.height / 2
      const distance = Math.hypot(bx - ax, by - ay)
      return document.elementFromPoint((ax + bx) / 2 - (by - ay) / distance * 4, (ay + by) / 2 + (bx - ax) / distance * 4)?.closest('[data-id]')?.getAttribute('data-id')
    })
    assert.equal(hit, 'test-line', '细线外侧 4 屏幕像素仍应能够选中线段')
    assert.equal(errors.length, 0, errors.join('; '))
    console.log('PASS: live event preview, cancel/save/reopen, length/angle, Shift endpoints, undo/redo, dash/cap/arrows, thin-line hit area, alignment overlay')
  } finally { await browser.close() }
}
main().catch(error => { console.error(error.message); process.exitCode = 1 })
