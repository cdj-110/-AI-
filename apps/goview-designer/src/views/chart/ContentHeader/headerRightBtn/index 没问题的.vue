<template>
  <n-space class="go-mt-0" :wrap="false">
    <n-button v-for="item in comBtnList" :key="item.title" :type="item.type" ghost @click="item.event">
      <template #icon>
        <component :is="item.icon"></component>
      </template>
      <span>{{ item.title }}</span>
    </n-button>
  </n-space>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { renderIcon, goDialog, fetchPathByName, routerTurnByPath, setSessionStorage, getSessionStorage } from '@/utils'
import { PreviewEnum } from '@/enums/pageEnum'
import { StorageEnum } from '@/enums/storageEnum'
import { useRoute } from 'vue-router'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { syncData } from '../../ContentEdit/components/EditTools/hooks/useSyncUpdate.hook'
import { icon } from '@/plugins'
import { cloneDeep } from 'lodash'
import { apiV1Config, configurationPointBindings } from '@/api'

let userId = ref(sessionStorage.getItem('wk_UserId') as any);
let wk_Config_id = ref(sessionStorage.getItem('wk_Config_id') as any);

const { BrowsersOutlineIcon, SendIcon, AnalyticsIcon } = icon.ionicons5
const chartEditStore = useChartEditStore()

const routerParamsInfo = useRoute()

// 预览
const previewHandle = () => {
  const path = fetchPathByName(PreviewEnum.CHART_PREVIEW_NAME, 'href')
  if (!path) return
  const { id } = routerParamsInfo.params
  // id 标识
  // const previewId = typeof id === 'string' ? id : id[0]
  const previewId = wk_Config_id.value
  const storageInfo = chartEditStore.getStorageInfo()
  const sessionStorageInfo = getSessionStorage(StorageEnum.GO_CHART_STORAGE_LIST) || []

  if (sessionStorageInfo?.length) {
    const repeateIndex = sessionStorageInfo.findIndex((e: { id: string }) => e.id === previewId)
    // 重复替换
    if (repeateIndex !== -1) {
      sessionStorageInfo.splice(repeateIndex, 1, { id: previewId, ...storageInfo })
      setSessionStorage(StorageEnum.GO_CHART_STORAGE_LIST, sessionStorageInfo)
    } else {
      sessionStorageInfo.push({
        id: previewId,
        ...storageInfo
      })
      setSessionStorage(StorageEnum.GO_CHART_STORAGE_LIST, sessionStorageInfo)
    }
  } else {
    setSessionStorage(StorageEnum.GO_CHART_STORAGE_LIST, [{ id: previewId, ...storageInfo }])
  }
  createConfigChange(chartEditStore.getStorageInfo());
  // 跳转
  routerTurnByPath(path, [previewId], undefined, true)
}

// 发布
const sendHandle = () => {
  goDialog({
    message: '是否确认发布此组态',
    positiveText: '确认发布',
    negativeText: "取消发布",
    onPositiveCallback: () => {
      createConfigChange(chartEditStore.getStorageInfo());
    }
  })
}

/**
 * 将组态内容保存到后端
 */
let createConfigChange = (list: any) => {
  let arr:Array<{}> = [];
  list.componentList.forEach((item: any) => {
    if(item.request.requestParams["Params"] && Array.isArray(item.request.requestParams["Params"])){
      item.request.requestParams["Params"].forEach((list: any) => {
        arr.push({ dev_id: list.value1, identifier: list.value2 });
      })
    }
  })
  configurationPointBindings({
    user_id: userId.value,
    config_id: wk_Config_id.value,
    bindings: arr
  }).then((res: any) => {
    if(res.code == 200 && res.msg == "ok"){
      apiV1Config({
        user_id: userId.value,
        config_id: wk_Config_id.value,
        username: "test",
        main_data: { item: Array.isArray(list) ? list : [list] }
      }).then((res:any) => {
        if(res.success){
          window['$message'].success(res.message);
          let listlist = JSON.parse(sessionStorage.getItem("zhutaineirong") as any);
          if(listlist){
            listlist.main_data.item = Array.isArray(list) ? list : [list];
            sessionStorage.setItem("zhutaineirong", JSON.stringify(listlist));
          }
        }
      })
    }else{
      window['$message'].error("点位存储失败，组态整体存储失败！");
    }
  })
}

const btnList = [
  {
    select: true,
    title: '同步内容',
    type: 'primary',
    icon: renderIcon(AnalyticsIcon),
    event: syncData
  },
  {
    select: true,
    title: '预览',
    icon: renderIcon(BrowsersOutlineIcon),
    event: previewHandle
  },
  {
    select: true,
    title: '发布',
    icon: renderIcon(SendIcon),
    event: sendHandle
  }
]

const comBtnList = computed(() => {
  if (chartEditStore.getEditCanvas.isCodeEdit) {
    return btnList
  }
  const cloneList = cloneDeep(btnList)
  cloneList.shift()
  return cloneList
})
</script>

<style lang="scss" scoped>
.align-center {
  margin-top: -4px;
}
</style>
