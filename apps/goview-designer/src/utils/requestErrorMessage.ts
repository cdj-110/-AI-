export function requestErrorMessage(error: any): string {
  const message = error?.response?.data?.message || error?.response?.data?.msg
  const text = Array.isArray(message) ? message.join('，') : typeof message === 'string' ? message : ''
  if (error?.response?.status === 413 || /request entity too large/i.test(text)) {
    return '请求内容过大，请减少配置或图片大小后重试'
  }
  if (error?.response?.status >= 500) return '服务器内部错误，请稍后重试'
  return text.slice(0, 500) || '请求失败，请稍后重试'
}
