type Descriptor = { key: string; package?: string; category?: string; redirectComponent?: string }
type Loader = () => Promise<{ default: any }>

/** 返回稳定的异步组件，不在注册阶段执行模块加载。 */
export function createLazyComponentRegistry(modules: Record<string, Loader>, wrap: (loader: Loader) => any) {
  const cache = new Map<string, any>()
  return (descriptor: Descriptor, file: 'index.vue' | 'config.vue') => {
    const exact = `./components/${descriptor.package}/${descriptor.category}/${descriptor.key}/${file}`
    const redirect = descriptor.redirectComponent && `./components/${descriptor.redirectComponent}/${file}`
    const path = modules[exact] ? exact : redirect && modules[redirect] ? redirect
      : Object.keys(modules).find(path => path.endsWith(`/${descriptor.key}/${file}`))
    if (!path) return undefined
    if (!cache.has(path)) cache.set(path, wrap(modules[path]))
    return cache.get(path)
  }
}
