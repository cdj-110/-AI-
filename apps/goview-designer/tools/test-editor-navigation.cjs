const assert = require('node:assert/strict'), fs = require('node:fs'), path = require('node:path'), ts = require('typescript')
const root = path.resolve(__dirname, '..'), result = { exports: {} }
new Function('exports', ts.transpileModule(fs.readFileSync(path.join(root, 'src/utils/editorNavigation.ts'), 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText)(result.exports)
const nav = result.exports
const data = { componentList: [{ id: 'text', attr: { x: 10, y: 20 }, option: { fontSize: 20 } }], editCanvasConfig: { width: 1920 }, requestGlobalConfig: {} }
const store = { pageCallList: [{ title: '画布1', ...data }, { title: '画布2', componentList: [] }], getCurrentCanvasTitle: '画布1', getStorageInfo: () => data }
nav.resetEditorNavigation(); assert.equal(nav.editorHasUnsavedChanges(store), false)
nav.markEditorLoaded(store); assert.equal(nav.editorHasUnsavedChanges(store), false)
data.componentList[0].attr.offsetX = 10; assert.equal(nav.editorHasUnsavedChanges(store), false)
data.componentList[0].option.fontSize = 30; assert.equal(nav.editorHasUnsavedChanges(store), true)
const submitted = nav.editorDocumentSignature(store)
data.componentList[0].option.fontSize = 40
nav.markEditorSaved(submitted); assert.equal(nav.editorHasUnsavedChanges(store), true, '保存期间新编辑不能被标记为已保存')
nav.markEditorSaved(nav.editorDocumentSignature(store)); assert.equal(nav.editorHasUnsavedChanges(store), false)
store.pageCallList[1].title = '第二页'; assert.equal(nav.editorHasUnsavedChanges(store), true)
nav.markEditorLoaded(store, true); assert.equal(nav.editorHasUnsavedChanges(store), true, '恢复草稿应提醒保存')
nav.markEditorLoaded(store)
data.componentList[0].option.dataset = { list: [1, 2] }
assert.equal(nav.editorHasUnsavedChanges(store), true, '不能忽略数据中的 list 字段')
assert.equal(nav.platformProjectsUrl('https://iot.example', null, 'http://127.0.0.1:8080'), 'https://iot.example/cloud-scada')
assert.equal(nav.platformProjectsUrl(null, 'http://127.0.0.1:5174/api', 'http://127.0.0.1:8080'), 'http://127.0.0.1:5174/cloud-scada')
assert.throws(() => nav.platformProjectsUrl('javascript:alert(1)', null, 'http://127.0.0.1:8080'))
assert.throws(() => nav.platformProjectsUrl(null, null, 'https://designer.example/'))
const { parse, compileScript } = require('@vue/compiler-sfc')
for (const file of ['src/components/GoEditorMenu/index.vue', 'src/layout/components/LayoutHeaderPro/index.vue', 'src/views/chart/ContentEdit/components/EditTools/index.vue']) compileScript(parse(fs.readFileSync(path.join(root, file), 'utf8')).descriptor, { id: 'editor-menu-test' })
console.log('PASS: dirty state, unchanged documents, canvas rename, recovered draft, in-flight save edits, safe platform destination, menu SFC compilation')
