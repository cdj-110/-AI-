const assert = require('node:assert/strict')
const { chromium } = require(process.argv[2] || 'playwright')
const base = process.argv[3] || 'http://127.0.0.1:3029'
async function main() {
  const browser = await chromium.launch({ channel: 'chrome', headless: true })
  try {
    const context = await browser.newContext({ viewport: { width: 1700, height: 1100 } })
    const page = await context.newPage(), errors = []
    page.on('pageerror', error => errors.push(error.message))
    await context.addInitScript(() => {
      sessionStorage.setItem('wk_Token', JSON.stringify({ token: 'synthetic' }))
      sessionStorage.setItem('wk_UserId', 'resize-user')
      sessionStorage.setItem('wk_Config_id', 'resize-test')
      sessionStorage.setItem('wk_ApiBaseUrl', 'http://127.0.0.1:5174/api')
    })
    function item(id, key, pkg, x, y, w, h, option, category) {
      return { id, key, isGroup: false, attr: { x, y, w, h, contentW: w, contentH: h, zIndex: -1 },
        chartConfig: { key, chartKey: `V${key}`, conKey: `VC${key}`, title: key, category, categoryName: category, package: pkg, chartFrame: key === 'BarCommon' ? 'echarts' : 'common', image: '' }, option, interactActions: [] }
    }
    const components = [
      item('text', 'TextCommon', 'Informations', 100, 120, 240, 100, { dataset: '缩放文字', fontSize: 20, fontColor: '#fff', paddingX: 10, paddingY: 10, textAlign: 'center', fontWeight: 'normal', borderWidth: 0, borderColor: '#fff', borderRadius: 5, letterSpacing: 1, writingMode: 'horizontal-tb', backgroundColor: '#00000000' }, 'Texts'),
      item('circle', 'CircleFun', 'BasicShapes', 650, 120, 180, 180, { borderColor: '#3774b6', backgroundColor: '#163d50', gradientColor: ['#163d50', '#163d50'], backgroundLinearNumber: 70, fillOpacity: 1, strokeOpacity: 1, borderType: 'solid', borderWidth: 3 }, 'Graph'),
      item('table', 'TablesBasic', 'Tables', 100, 580, 600, 260, { dataset: { dimensions: [{ title: '数据', key: 'value' }], source: [{ value: 42 }] }, pagination: { page: 1, pageSize: 5 }, align: 'center', style: { border: 'on', singleColumn: 'off', singleLine: 'off', bottomBordered: 'on', striped: 'on', fontSize: 16, borderWidth: 0, borderColor: 'black', borderStyle: 'solid' }, inputShow: 'none' }, 'Tables'),
      item('bar', 'BarCommon', 'Charts', 850, 150, 360, 240, { xAxis: { type: 'category' }, yAxis: { type: 'value' }, dataset: { dimensions: ['name', 'value'], source: [['A', 10], ['B', 20]] }, series: [{ type: 'bar', label: { show: true, fontSize: 12 } }] }, 'Bars')
    ]
    await context.route('**/api/**', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(route.request().url().includes('/auth/activity') ? { data: { accessToken: 'synthetic-fresh', idleTimeoutSeconds: 3600 } } : { main_data: { item: [{ title: '画布1', componentList: components }] } }) }))
    await page.goto(`${base}/#/chart/home/resize-test`, { waitUntil: 'domcontentloaded' })
    const shape = id => page.locator(`[data-id="${id}"]`)
    await shape('text').waitFor({ state: 'visible', timeout: 30000 })
    await page.waitForTimeout(1000)
    const measure = id => shape(id).evaluate(element => {
      const visual = element.querySelector('.edit-content-chart'), rect = element.getBoundingClientRect(), v = visual.getBoundingClientRect()
      const text = element.querySelector('.content')
      return { x: rect.x, y: rect.y, w: rect.width, h: rect.height, vx: v.x, vy: v.y, vw: v.width, vh: v.height, font: text ? parseFloat(getComputedStyle(text).fontSize) : null }
    })
    const close = (a, b, tolerance = 1.5) => assert.ok(Math.abs(a - b) < tolerance, `${a} != ${b}`)
    async function drag(id, point, dx, dy, modifiers = []) {
      await shape(id).click({ position: { x: 20, y: 20 } })
      await page.waitForTimeout(250)
      const rect = await shape(id).locator(`.shape-point.${point}`).boundingBox()
      const hit = await page.evaluate(({ x, y }) => document.elementFromPoint(x, y)?.outerHTML.slice(0, 350), { x: rect.x + rect.width / 2, y: rect.y + rect.height / 2 })
      assert.ok(hit?.includes(`shape-point ${point}`), JSON.stringify({ id, rect, hit }))
      for (const key of modifiers) await page.keyboard.down(key)
      await page.mouse.move(rect.x + rect.width / 2, rect.y + rect.height / 2); await page.mouse.down()
      await page.mouse.move(rect.x + rect.width / 2 + dx, rect.y + rect.height / 2 + dy, { steps: 8 }); await page.mouse.up()
      for (const key of [...modifiers].reverse()) await page.keyboard.up(key)
      await page.waitForTimeout(150)
    }
    const initial = await measure('text')
    await drag('text', 'rb', 50, 25)
    const normal = await measure('text')
    assert.ok(normal.vw > initial.vw + 40, JSON.stringify({ initial, normal })); assert.ok(normal.font > 20); close(normal.w, normal.vw)
    await page.keyboard.press('Control+z'); await page.waitForTimeout(100)
    const undone = await measure('text'); close(undone.w, initial.w); assert.equal(undone.font, 20)
    await page.keyboard.press('Control+Shift+z'); await page.waitForTimeout(100)
    close((await measure('text')).font, normal.font, .1)
    await page.keyboard.press('Control+z'); await page.waitForTimeout(100)
    await drag('text', 'lt', -45, -20, ['Control'])
    const container = await measure('text')
    assert.ok(container.w > initial.w + 35); close(container.vw, initial.vw); close(container.vx, initial.vx); close(container.vy, initial.vy); assert.equal(container.font, 20)
    await page.keyboard.press('Control+z'); await page.waitForTimeout(100)
    await drag('text', 'rb', 50, 3, ['Shift'])
    const proportional = await measure('text'); close(proportional.w / proportional.h, initial.w / initial.h, .02); assert.ok(proportional.font > 20)
    await page.keyboard.press('Control+z'); await page.waitForTimeout(100)
    await drag('text', 'rb', 50, 3, ['Control', 'Shift'])
    const only = await measure('text'); close(only.w / only.h, initial.w / initial.h, .02); close(only.vw, initial.vw); assert.equal(only.font, 20)
    await drag('circle', 'rb', 45, 2)
    const circle = await measure('circle'); close(circle.w, circle.h)
    const beforeTable = await measure('table'), tableFont = await shape('table').locator('.n-data-table').evaluate(e => getComputedStyle(e).fontSize)
    await drag('table', 'r', 40, 0)
    const afterTable = await measure('table'); assert.ok(afterTable.vw > beforeTable.vw + 30); close(afterTable.h, beforeTable.h)
    assert.equal(await shape('table').locator('.n-data-table').evaluate(e => getComputedStyle(e).fontSize), tableFont)
    const beforeBar = await measure('bar')
    await drag('bar', 'rb', 30, 20)
    const afterBar = await measure('bar'); assert.ok(afterBar.vw > beforeBar.vw + 20)
    await page.waitForTimeout(300)
    const canvas = await shape('bar').locator('canvas, svg').first().boundingBox(); close(canvas.width, afterBar.vw)
    // 移动对齐隐藏编辑器边框与锚点，松手恢复；缩放阶段仍保留边框。
    await shape('circle').click({ position: { x: 20, y: 20 } })
    const moving = await shape('circle').boundingBox()
    await page.mouse.move(moving.x + 20, moving.y + 20); await page.mouse.down()
    await page.mouse.move(moving.x + 35, moving.y + 20, { steps: 3 }); await page.waitForTimeout(80)
    assert.ok(await shape('circle').evaluate(e => e.classList.contains('drag-aligning')))
    assert.equal(await shape('circle').locator('.shape-modal').evaluate(e => getComputedStyle(e).visibility), 'hidden')
    assert.equal(await shape('circle').locator('.shape-point').first().evaluate(e => getComputedStyle(e).visibility), 'hidden')
    assert.equal(await page.locator('.go-edit-align-line').evaluate(e => getComputedStyle(e).visibility), 'visible')
    assert.ok(await page.locator('.go-edit-align-line .highlight-line, .go-edit-align-line .guide-line').count() > 0, '移动对齐期间应保留辅助线')
    await page.mouse.up(); await page.waitForTimeout(80)
    assert.equal(await shape('circle').locator('.shape-modal').evaluate(e => getComputedStyle(e).visibility), 'visible')
    await shape('text').click({ position: { x: 20, y: 20 } })
    await page.waitForTimeout(250)
    const handle = await shape('text').locator('.shape-point.rb').boundingBox()
    const handleHit = await page.evaluate(({ x, y }) => document.elementFromPoint(x, y)?.outerHTML.slice(0, 200), { x: handle.x + handle.width / 2, y: handle.y + handle.height / 2 })
    assert.ok(handleHit?.includes('shape-point rb'), JSON.stringify({ handle, handleHit }))
    await page.mouse.move(handle.x + handle.width / 2, handle.y + handle.height / 2); await page.mouse.down()
    await page.mouse.move(handle.x + handle.width / 2 + 5, handle.y + handle.height / 2 + 5); await page.waitForTimeout(80)
    assert.equal(await shape('text').locator('.shape-modal').evaluate(e => getComputedStyle(e).visibility), 'visible')
    await page.mouse.up()
    assert.deepEqual(errors, [])
    console.log('PASS: component resize/modifiers, reflow, undo/redo, alignment hides frame/handles and restores on release, resize retains frame; no page errors')
  } finally { await browser.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
