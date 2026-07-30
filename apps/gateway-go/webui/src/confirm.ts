import { reactive, readonly } from 'vue'

export type ConfirmOptions = {
  title?: string
  message: string
  detail?: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
}

const state = reactive({
  open: false,
  title: '确认操作',
  message: '',
  detail: '',
  confirmText: '确认',
  cancelText: '取消',
  danger: false,
})

let resolvePending: ((confirmed: boolean) => void) | undefined

export function confirmAction(options: ConfirmOptions | string): Promise<boolean> {
  if (resolvePending) resolvePending(false)
  const next: ConfirmOptions = typeof options === 'string' ? { message: options } : options
  Object.assign(state, {
    open: true,
    title: next.title || '确认操作',
    message: next.message,
    detail: next.detail || '',
    confirmText: next.confirmText || '确认',
    cancelText: next.cancelText || '取消',
    danger: next.danger === true,
  })
  return new Promise<boolean>(resolve => {
    resolvePending = resolve
  })
}

export function closeConfirm(confirmed: boolean) {
  if (!state.open) return
  state.open = false
  const resolve = resolvePending
  resolvePending = undefined
  resolve?.(confirmed)
}

export function useConfirmState() {
  return readonly(state)
}
