type Editor = { pageCallList: any[]; getCurrentCanvasTitle: string; getStorageInfo(): any }
let savedSignature: string | null = null
let ready = false

export function editorDocumentSignature(editor: Editor) {
  const title = editor.getCurrentCanvasTitle || '画布1'
  const pages = editor.pageCallList.map(page => {
    const { list, ...pageData } = page
    return { ...pageData, ...list }
  })
  const current = { title, ...editor.getStorageInfo() }
  const index = pages.findIndex(page => page.title === title)
  if (index < 0) pages.push(current)
  else pages[index] = current
  const stable = (value: any, parentKey = ''): any => {
    if (Array.isArray(value)) return value.map(item => stable(item))
    if (!value || typeof value !== 'object') return value
    return Object.fromEntries(Object.keys(value).sort().filter(key => parentKey !== 'attr' || !['offsetX', 'offsetY', 'zIndex'].includes(key)).map(key => [key, stable(value[key], key)]))
  }
  return JSON.stringify(stable(pages))
}
export function resetEditorNavigation() { ready = false; savedSignature = null }
export function markEditorLoaded(editor: Editor, restoredDraft = false) { ready = true; savedSignature = restoredDraft ? null : editorDocumentSignature(editor) }
export function markEditorSaved(signature: string) { ready = true; savedSignature = signature }
export function editorHasUnsavedChanges(editor: Editor) { return ready && (savedSignature === null || editorDocumentSignature(editor) !== savedSignature) }

export function platformProjectsUrl(platformOrigin: string | null, apiBase: string | null, currentUrl: string) {
  const configured = platformOrigin || apiBase
  let origin: URL
  if (configured) origin = new URL(configured)
  else {
    origin = new URL(currentUrl)
    if (!['localhost', '127.0.0.1', '[::1]'].includes(origin.hostname)) throw new Error('未配置平台地址，请从平台重新进入编辑器')
    origin.port = '5174'
  }
  if (!['http:', 'https:'].includes(origin.protocol)) throw new Error('平台地址无效')
  return new URL('/cloud-scada', origin.origin).href
}
