import { reactive, readonly } from 'vue'

export type ConfirmOptions = {
  title?: string
  message: string
  detail?: string
  confirmText?: string
  cancelText?: string
  discardText?: string
  danger?: boolean
  kind?: 'license'
  fields?: {label:string;value:string}[]
}

const state = reactive({
  open: false,
  title: '确认操作',
  message: '',
  detail: '',
  confirmText: '确认',
  cancelText: '取消',
  discardText: '',
  danger: false,
  kind: '' as '' | 'license',
  fields: [] as {label:string;value:string}[],
})

let resolvePending: ((confirmed: boolean | 'discard') => void) | undefined

export function confirmAction(options: ConfirmOptions | string): Promise<boolean> {
  return openConfirm(options).then(result=>result===true)
}

export function confirmDraftAction(): Promise<boolean | 'discard'> {
  return openConfirm({title:'尚有未保存的修改',message:'是否保存修改后继续？',detail:'保存失败时会保留修改并停留在当前页面。',confirmText:'保存修改并继续',cancelText:'继续编辑',discardText:'放弃修改并离开'})
}

function openConfirm(options: ConfirmOptions | string): Promise<boolean | 'discard'> {
  if (resolvePending) resolvePending(false)
  const next: ConfirmOptions = typeof options === 'string' ? { message: options } : options
  Object.assign(state, {
    open: true,
    title: next.title || '确认操作',
    message: next.message,
    detail: next.detail || '',
    confirmText: next.confirmText || '确认',
    cancelText: next.cancelText || '取消',
    discardText: next.discardText || '',
    danger: next.danger === true,
    kind: next.kind || '',
    fields: next.fields ? next.fields.map(field=>({...field})) : [],
  })
  return new Promise<boolean | 'discard'>(resolve => {
    resolvePending = resolve
  })
}

export function closeConfirm(confirmed: boolean | 'discard') {
  if (!state.open) return
  state.open = false
  const resolve = resolvePending
  resolvePending = undefined
  resolve?.(confirmed)
}

export function useConfirmState() {
  return readonly(state)
}
