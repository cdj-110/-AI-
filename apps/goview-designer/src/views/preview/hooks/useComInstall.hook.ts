import { ref } from 'vue'
import { ChartEditStorageType } from '../index.d'
import { CreateComponentType, CreateComponentGroupType } from '@/packages/index.d'
import { fetchChartComponent } from '@/packages/index'

export const installPreviewComponents = (componentList: Array<CreateComponentType | CreateComponentGroupType> = []) => {
  const app = window['$vue']
  if (!app?.component) return false

  const installComponent = (target: CreateComponentType) => {
    const chartKey = target?.chartConfig?.chartKey
    if (!chartKey || app.component(chartKey)) return
    const component = fetchChartComponent(target.chartConfig)
    if (component) app.component(chartKey, component)
  }

  componentList.forEach((item) => {
    if (item?.isGroup && Array.isArray((item as CreateComponentGroupType).groupList)) {
      ;(item as CreateComponentGroupType).groupList.forEach(installComponent)
    } else {
      installComponent(item as CreateComponentType)
    }
  })

  return true
}

export const useComInstall = (localStorageInfo: ChartEditStorageType) => {
  const show = ref(false)

  // 注册组件(一开始无法获取window['$vue'])
  const intervalTiming = setInterval(() => {
    if (installPreviewComponents(localStorageInfo.componentList || [])) {
      clearInterval(intervalTiming)
      show.value = true
    }
  }, 200)

  return {
    show
  }
}
