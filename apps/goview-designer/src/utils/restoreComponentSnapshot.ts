// 撤销时保留响应式对象引用，兼容在 setup 中通过 toRefs 读取属性的组件。
export function restoreComponentSnapshot(target: Record<string, any>, snapshot: Record<string, any>) {
  for (const key of Object.keys(target)) if (!(key in snapshot)) delete target[key]
  for (const [key, value] of Object.entries(snapshot)) {
    if (value && typeof value === 'object' && !Array.isArray(value) && target[key] && typeof target[key] === 'object' && !Array.isArray(target[key])) {
      restoreComponentSnapshot(target[key], value)
    } else target[key] = value
  }
}
