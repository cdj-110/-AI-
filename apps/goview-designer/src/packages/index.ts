import { ChartList } from '@/packages/components/Charts/index'
import { VChartList } from '@/packages/components/VChart/index'
import { DecorateList } from '@/packages/components/Decorates/index'
import { BasicShapes } from '@/packages/components/BasicShapes/index'
import { InformationList } from '@/packages/components/Informations/index'
import { TableList } from '@/packages/components/Tables/index'
import { PhotoList } from '@/packages/components/Photos/index'
import { IconList } from '@/packages/components/Icons/index'
import { PackagesCategoryEnum, PackagesType, ConfigType, FetchComFlagType } from '@/packages/index.d'
import { initializeComponentBounds } from '@/utils/componentInitialBounds'
import { defineAsyncComponent } from 'vue'
import { createLazyComponentRegistry } from '@/utils/lazyComponentRegistry'

const configModules = import.meta.glob<{ default: any }>('./components/**/config.vue')
const indexModules = import.meta.glob<{ default: any }>('./components/**/index.vue')
const lazyComponent = createLazyComponentRegistry({ ...indexModules, ...configModules }, loader =>
  defineAsyncComponent({ loader: async () => (await loader()).default, delay: 120, timeout: 30_000 })
)
const imagesModules: Record<string, { default: string }> = import.meta.glob('../assets/images/chart/**', {
  eager: true
})

// 新建组件时让内容尺寸与容器尺寸完全贴合。后续普通拖角只改变容器，
// Ctrl + 拖角才会同步改变内容尺寸。
const initializeComponentContentSize = (instance: CreateComponentType) => {
  return initializeComponentBounds(instance)
}

// * 所有图表
export let packagesList: PackagesType = {
  [PackagesCategoryEnum.BasicShapes]: BasicShapes,
  [PackagesCategoryEnum.CHARTS]: ChartList,
  [PackagesCategoryEnum.VCHART]: VChartList,
  [PackagesCategoryEnum.INFORMATIONS]: InformationList,
  [PackagesCategoryEnum.TABLES]: TableList,
  [PackagesCategoryEnum.DECORATES]: DecorateList,
  [PackagesCategoryEnum.PHOTOS]: PhotoList,
  [PackagesCategoryEnum.ICONS]: IconList
}

// 组件缓存, 可以大幅度提升组件加载速度
const componentCacheMap = new Map<string, any>()
const loadConfig = async (packageName: string, categoryName: string, keyName: string) => {
  const key = packageName + categoryName + keyName
  
  if (!componentCacheMap.has(key)) {
    try {
      const module = await import(`./components/${packageName}/${categoryName}/${keyName}/config.ts`)
      componentCacheMap.set(key, module)
      return module
    } catch (error) {
      console.error(`加载组件配置失败: ${packageName}/${categoryName}/${keyName}`, error)
      return null
    }
  }
  
  const cached = componentCacheMap.get(key)
  if (cached && typeof cached.then === 'function') {
    try {
      const module = await cached
      componentCacheMap.set(key, module)
      return module
    } catch (error) {
      console.error(`加载缓存组件配置失败`, error)
      componentCacheMap.delete(key)
      return null
    }
  }
  
  return cached
}

/**
 * * 获取目标组件配置信息
 * @param targetData
 */
export const createComponent = async (targetData: ConfigType): Promise<CreateComponentType | null> => {
  if (!targetData) {
    console.error('createComponent: targetData 为空')
    throw new Error('组件配置数据为空')
  }
  
  const { redirectComponent, category, key, package: packageName } = targetData
  
  // 验证必要字段
  if (!key) {
    console.error('createComponent: key 为空', targetData)
    throw new Error('组件 key 缺失')
  }
  
  if (!category) {
    console.error('createComponent: category 为空', targetData)
    throw new Error(`组件分类缺失: key=${key}`)
  }
  
  // redirectComponent 是给图片组件库和图标组件库使用的
  if (redirectComponent) {
    const parts = redirectComponent.split('/')
    if (parts.length !== 3) {
      console.error('createComponent: redirectComponent 格式错误', redirectComponent)
      throw new Error(`redirectComponent 格式错误: ${redirectComponent}`)
    }
    const [pkgName, catName, keyName] = parts
    const redirectChart = await loadConfig(pkgName, catName, keyName)
    
    if (!redirectChart || !redirectChart.default) {
      console.error('createComponent: 无法加载重定向组件配置', redirectComponent)
      throw new Error(`无法加载组件配置: ${redirectComponent}`)
    }
    
    try {
      const instance = new redirectChart.default()
      if (!instance || !instance.attr) {
        console.error('createComponent: 重定向组件实例化失败', redirectComponent)
        throw new Error(`组件实例化失败: ${redirectComponent}`)
      }
      return initializeComponentContentSize(instance)
    } catch (error) {
      console.error('createComponent: 重定向组件创建异常', redirectComponent, error)
      throw new Error(`组件创建异常: ${(error as Error).message}`)
    }
  }
  
  if (!packageName) {
    console.error('createComponent: packageName 为空', targetData)
    throw new Error(`组件包名缺失: key=${key}`)
  }
  
  const chart = await loadConfig(packageName, category, key)
  if (!chart || !chart.default) {
    console.error('createComponent: 无法加载组件配置', packageName, category, key)
    throw new Error(`无法加载组件配置: ${packageName}/${category}/${key}`)
  }
  
  try {
    const instance = new chart.default()
    if (!instance || !instance.attr) {
      console.error('createComponent: 组件实例化失败', key)
      throw new Error(`组件实例化失败: ${key}`)
    }
    return initializeComponentContentSize(instance)
  } catch (error) {
    console.error('createComponent: 组件创建异常', key, error)
    throw new Error(`组件创建异常: ${(error as Error).message}`)
  }
}

/**
 * * 获取组件
 * @param {string} chartName 名称
 * @param {FetchComFlagType} flag 标识 0为展示组件, 1为配置组件
 */
const fetchComponent = (descriptor: ConfigType, flag: FetchComFlagType) =>
  lazyComponent(descriptor, flag === FetchComFlagType.VIEW ? 'index.vue' : 'config.vue')

/**
 * * 获取展示组件
 * @param {ConfigType} dropData 配置项
 */
export const fetchChartComponent = (dropData: ConfigType) => {
  return fetchComponent(dropData, FetchComFlagType.VIEW)
}

/**
 * * 获取配置组件
 * @param {ConfigType} dropData 配置项
 */
export const fetchConfigComponent = (dropData: ConfigType) => {
  return fetchComponent(dropData, FetchComFlagType.CONFIG)
}

/**
 * * 获取图片内容
 * @param {ConfigType} targetData 配置项
 */
export const fetchImages = async (targetData?: ConfigType) => {
  if (!targetData) return ''
  
  const { image } = targetData
  
  if (!image || typeof image !== 'string') {
    console.warn('fetchImages: image 字段为空或不是字符串', targetData)
    return ''
  }
  
  if (/^(http|https):\/\/([\w.]+\/?)\S*/.test(image)) return image
  
  if (image.includes('@') || image.includes('base64')) return image

  const imageName = image.substring(image.lastIndexOf('/') + 1)
  for (const key in imagesModules) {
    const urlSplit = key.split('/')
    if (urlSplit[urlSplit.length - 1] === imageName) {
      return imagesModules[key]?.default
    }
  }
  return ''
}
