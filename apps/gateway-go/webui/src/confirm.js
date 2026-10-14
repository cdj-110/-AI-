import { reactive, readonly } from 'vue';
const state = reactive({
    open: false,
    title: '确认操作',
    message: '',
    detail: '',
    confirmText: '确认',
    cancelText: '取消',
    danger: false,
});
let resolvePending;
export function confirmAction(options) {
    if (resolvePending)
        resolvePending(false);
    const next = typeof options === 'string' ? { message: options } : options;
    Object.assign(state, {
        open: true,
        title: next.title || '确认操作',
        message: next.message,
        detail: next.detail || '',
        confirmText: next.confirmText || '确认',
        cancelText: next.cancelText || '取消',
        danger: next.danger === true,
    });
    return new Promise(resolve => {
        resolvePending = resolve;
    });
}
export function closeConfirm(confirmed) {
    if (!state.open)
        return;
    state.open = false;
    const resolve = resolvePending;
    resolvePending = undefined;
    resolve?.(confirmed);
}
export function useConfirmState() {
    return readonly(state);
}
