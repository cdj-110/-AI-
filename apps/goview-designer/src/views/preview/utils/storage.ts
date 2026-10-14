import { getSessionStorage } from '@/utils'
import { StorageEnum } from '@/enums/storageEnum'
import { ChartEditStorage } from '@/store/modules/chartEditStore/chartEditStore.d'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { apiV1configDatas_test } from '@/api/index'
import { ref } from 'vue'

const chartEditStore = useChartEditStore()

export interface ChartEditStorageType extends ChartEditStorage {
  id: string
  canvasTitle?: string
}

export const getSessionStorageInfo = async () => {
  const hash = window.location.hash;
  const hashParams = new URLSearchParams(hash.split('?')[1] || '');
  
  const userId = hashParams.get('user_id') || sessionStorage.getItem('wk_UserId') || '';
  const configId = hash.split('/')[3]?.split('?')[0] || hashParams.get('config_id') || sessionStorage.getItem('wk_Config_id') || '';
  
  const pathParts = hash.split('/');
  const encodedCanvasTitle = pathParts[4]?.split('?')[0] || hashParams.get('canvasTitle') || '画布1';
  const canvasTitle = decodeURIComponent(encodedCanvasTitle);
  // 预览页必须以路由中的画布名作为当前画布。Pinia 的默认值是“画布1”，
  // 若直接打开画布2而不更新这里，后续会把画布2内容误写进画布1缓存。
  chartEditStore.setCurrentCanvasTitle(canvasTitle)

  const sessionStorageInfo = getSessionStorage(StorageEnum.GO_CHART_STORAGE_LIST) || []
  
  if (sessionStorageInfo.length > 0) {
    const matchedItem = sessionStorageInfo.find((item: any) => item.id === configId && item.canvasTitle === canvasTitle)
    if (matchedItem) {
      const { editCanvasConfig, requestGlobalConfig, componentList } = matchedItem
      chartEditStore.editCanvasConfig = editCanvasConfig
      chartEditStore.requestGlobalConfig = requestGlobalConfig
      chartEditStore.componentList = componentList
      return matchedItem
    }
  }

  let res:any = await apiV1configDatas_test({
    user_id: userId, 
    config_id: configId
  });
  let storageList = res.main_data.item
  
  if (storageList && storageList.length > 0) {
    let targetCanvas = storageList[0]
    
    const targetIndex = storageList.findIndex((item: any) => {
      return item.title === canvasTitle || item.title === canvasTitle
    })
    if (targetIndex !== -1) {
      targetCanvas = storageList[targetIndex]
    }
    
    if (targetCanvas) {
      targetCanvas = targetCanvas.list || targetCanvas
      let { editCanvasConfig, requestGlobalConfig, componentList } = targetCanvas
      chartEditStore.editCanvasConfig = editCanvasConfig
      chartEditStore.requestGlobalConfig = requestGlobalConfig
      chartEditStore.componentList = componentList
      return targetCanvas
    }
  }
}
