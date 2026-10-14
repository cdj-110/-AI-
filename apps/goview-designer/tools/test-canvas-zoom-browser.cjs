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
      const token = `synthetic.${btoa(JSON.stringify({ sub: 'zoom-user', tenantId: 'zoom-tenant' }))}.signature`
      sessionStorage.setItem('wk_Token', JSON.stringify({ token }))
      sessionStorage.setItem('wk_UserId', 'zoom-user')
      sessionStorage.setItem('wk_Config_id', 'zoom-test')
      sessionStorage.setItem('wk_ApiBaseUrl', 'http://127.0.0.1:5174/api')
    })
    await context.route('**/api/**', route => {
      const url = route.request().url()
      const body = url.includes('/auth/activity')
        ? { data: { accessToken: 'synthetic-fresh', idleTimeoutSeconds: 3600 } }
        : { main_data: { item: [] } }
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) })
    })
    await page.goto(`${base}/#/chart/home/zoom-test`, { waitUntil: 'domcontentloaded' })
    await page.locator('.go-edit-range').waitFor({ state: 'visible', timeout: 30000 })
    const results = await page.evaluate(async () => {
      const viewport = document.querySelector('.edit-screens')
      const canvas = document.querySelector('.go-edit-range')
      const content = document.querySelector('#go-chart-edit-content')
      const frame = () => new Promise(resolve => requestAnimationFrame(resolve))
      for (let i = 0; i < 35; i++) await frame()
      const results = []
      for (const atBoundary of [false, true]) {
        if (atBoundary) { viewport.scrollLeft = 0; viewport.scrollTop = 0; await frame() }
        const rect = viewport.getBoundingClientRect()
        const pointer = { x: rect.left + rect.width * .33, y: rect.top + rect.height * .42 }
        const initial = canvas.getBoundingClientRect()
        const initialScale = new DOMMatrix(getComputedStyle(canvas).transform).a
        const world = { x: (pointer.x - initial.left) / initialScale, y: (pointer.y - initial.top) / initialScale }
        let maxDrift = 0, samples = 0
        for (let i = 0; i < 28; i++) {
          // 一帧内多个滚轮事件，覆盖事件合并及连续放大/缩小。
          if (i < 20) for (let n = 0; n < 3; n++) viewport.dispatchEvent(new WheelEvent('wheel', {
            bubbles: true, cancelable: true, ctrlKey: true,
            clientX: pointer.x, clientY: pointer.y, deltaY: i < 10 ? -12 : 12
          }))
          await frame()
          const current = canvas.getBoundingClientRect()
          const renderedScale = new DOMMatrix(getComputedStyle(canvas).transform).a
          maxDrift = Math.max(maxDrift, Math.abs(current.left + world.x * renderedScale - pointer.x), Math.abs(current.top + world.y * renderedScale - pointer.y))
          samples++
        }
        results.push({ atBoundary, maxDrift, samples })
      }
      return { results, canvasTransition: getComputedStyle(canvas).transitionProperty, contentTransition: getComputedStyle(content).transitionProperty }
    })
    assert.equal(errors.length, 0, errors.join('; '))
    for (const result of results.results) assert.ok(result.maxDrift < 1, `缩放锚点抖动 ${JSON.stringify(result)}`)
    assert.ok(!results.canvasTransition.includes('transform') && !results.canvasTransition.includes('all'))
    assert.ok(!results.contentTransition.includes('width') && !results.contentTransition.includes('all'))
    console.log('PASS: frame-by-frame mouse anchor stable during wheel bursts and scroll boundaries', JSON.stringify(results.results))
  } finally { await browser.close() }
}
main().catch(error => { console.error(error.message); process.exitCode = 1 })
