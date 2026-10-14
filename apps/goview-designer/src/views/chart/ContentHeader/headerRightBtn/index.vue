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
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { renderIcon, goDialog, fetchPathByName, routerTurnByPath, setSessionStorage, getSessionStorage, JSONParse, JSONStringify, deepClone } from '@/utils'
import { PreviewEnum } from '@/enums/pageEnum'
import { StorageEnum } from '@/enums/storageEnum'
import { useRoute } from 'vue-router'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { syncData } from '../../ContentEdit/components/EditTools/hooks/useSyncUpdate.hook'
import { icon } from '@/plugins'
import { cloneDeep } from 'lodash'
import { apiV1Config, configurationPointBindings } from '@/api'
import { seriesItem } from "@/packages/components/Charts/Bars/BarCommon/config";
import html2canvas from 'html2canvas'
import { recoveryDraftKey } from '@/utils/sessionRecovery'
import { editorDocumentSignature, markEditorSaved } from '@/utils/editorNavigation'

let userId = ref(sessionStorage.getItem('wk_UserId') as any);
let wk_Config_id = ref(sessionStorage.getItem('wk_Config_id') as any);

const { BrowsersOutlineIcon, SendIcon, AnalyticsIcon } = icon.ionicons5
const chartEditStore = useChartEditStore()

const routerParamsInfo = useRoute()

onMounted(() => {
  window.addEventListener("keydown", handleKeydown);
})

onUnmounted(() => {
  window.removeEventListener("keydown", handleKeydown);
})

/**
 * 监听同时按下ctrl+s
 */
function handleKeydown(event: any) {
  if (event.ctrlKey && event.key === 's') {
    event.preventDefault();
    createConfigChange(chartEditStore.pageCallList);
  }
}

// 预览
const previewHandle = () => {
  const path = fetchPathByName(PreviewEnum.CHART_PREVIEW_NAME, 'href')
  if (!path) return
  const { id } = routerParamsInfo.params
  const previewId = wk_Config_id.value
  
  const canvasTitle = chartEditStore.getCurrentCanvasTitle || '画布1'
  
  const storageInfo = chartEditStore.getStorageInfo()
  const sessionStorageInfo = getSessionStorage(StorageEnum.GO_CHART_STORAGE_LIST) || []

  if (sessionStorageInfo?.length) {
    const repeateIndex = sessionStorageInfo.findIndex((e: { id: string; canvasTitle: string }) => e.id === previewId && e.canvasTitle === canvasTitle)
    if (repeateIndex !== -1) {
      sessionStorageInfo.splice(repeateIndex, 1, { id: previewId, canvasTitle, ...storageInfo })
      setSessionStorage(StorageEnum.GO_CHART_STORAGE_LIST, sessionStorageInfo)
    } else {
      sessionStorageInfo.push({
        id: previewId,
        canvasTitle,
        ...storageInfo
      })
      setSessionStorage(StorageEnum.GO_CHART_STORAGE_LIST, sessionStorageInfo)
    }
  } else {
    setSessionStorage(StorageEnum.GO_CHART_STORAGE_LIST, [{ id: previewId, canvasTitle, ...storageInfo }])
  }
  
  createConfigChange(chartEditStore.pageCallList);
  const encodedTitle = encodeURIComponent(canvasTitle)
  routerTurnByPath(path, [previewId, encodedTitle], undefined, true)
}

// 保存
const sendHandle = () => {
  goDialog({
    message: '是否确认保存此组态',
    positiveText: '确认保存',
    negativeText: "取消保存",
    onPositiveCallback: () => {
      // createConfigChange(chartEditStore.getStorageInfo());
      createConfigChange(chartEditStore.pageCallList);
    }
  })
}

/**
 * 解决绑定实时数据之后有多根柱子
 */
let shi_shi_zhu_zi = (item: any) => {
  //这个判断是为了解决绑定实时数据之后有多根柱子
  if(item.option && item.option.dataset && Object.prototype.toString.call(item.option.dataset) == "[object Object]" && Array.isArray(item.option.series) && item.request?.requestParams?.["Params"] && Array.isArray(item.request.requestParams["Params"])){
    if(item.option.series.length < item.request.requestParams["Params"].length){
      for(let iiv = 0; iiv < item.request.requestParams["Params"].length - item.option.series.length; iiv++){
        item.option.series.push(JSONParse(JSONStringify(item.option.series[0])));
      }
    }else if(item.option.series.length > item.request.requestParams["Params"].length){
      item.option.series = item.option.series.slice(0,item.request.requestParams["Params"].length);
    }
  }
}

/**
 * 将组态内容保存到后端
 */
const captureCanvasThumbnail = async () => {
  const canvasElement = document.querySelector('.go-edit-range') as HTMLElement | null
  if (!canvasElement) return ''

  try {
    const sourceCanvas = await html2canvas(canvasElement, {
      backgroundColor: chartEditStore.getEditCanvasConfig.background || '#081225',
      useCORS: true,
      allowTaint: false,
      logging: false,
      scale: 0.6,
      width: chartEditStore.getEditCanvasConfig.width,
      height: chartEditStore.getEditCanvasConfig.height,
      onclone: clonedDocument => {
        const clonedCanvas = clonedDocument.querySelector('.go-edit-range') as HTMLElement | null
        if (clonedCanvas) {
          clonedCanvas.style.transform = 'none'
          clonedCanvas.style.transformOrigin = 'left top'
          // 编辑时允许画布外组件可见，缩略图仍只截取画布。
          clonedCanvas.style.overflow = 'hidden'
        }
        clonedDocument.querySelectorAll('.go-edit-range-model, .go-edit-select, .go-edit-align-line').forEach(node => {
          ;(node as HTMLElement).style.display = 'none'
        })
      }
    })

    const width = 640
    const height = 360
    const thumbnailCanvas = document.createElement('canvas')
    thumbnailCanvas.width = width
    thumbnailCanvas.height = height
    const context = thumbnailCanvas.getContext('2d')
    if (!context) return ''

    context.fillStyle = chartEditStore.getEditCanvasConfig.background || '#081225'
    context.fillRect(0, 0, width, height)
    const ratio = Math.min(width / sourceCanvas.width, height / sourceCanvas.height)
    const drawWidth = sourceCanvas.width * ratio
    const drawHeight = sourceCanvas.height * ratio
    context.drawImage(sourceCanvas, (width - drawWidth) / 2, (height - drawHeight) / 2, drawWidth, drawHeight)
    return thumbnailCanvas.toDataURL('image/jpeg', 0.72)
  } catch (error) {
    console.warn('生成组态预览图失败，将继续保存组态配置', error)
    return ''
  }
}

let createConfigChange = async (list: any) => {
  const canvasTitle = chartEditStore.getCurrentCanvasTitle || '画布1'
  const currentPage = { title: canvasTitle, ...JSONParse(JSONStringify(chartEditStore.getStorageInfo())) }
  const currentIndex = chartEditStore.pageCallList.findIndex(item => item.title === canvasTitle)
  if (currentIndex !== -1) {
    chartEditStore.pageCallList[currentIndex] = currentPage;
  } else {
    chartEditStore.pageCallList.push(currentPage);
  }
  list = chartEditStore.pageCallList
 
  let arr:Array<{}> = [];

  let verTeplte = list.map((teplte: any) => {
    if(!teplte.componentList || teplte.componentList.filter((item: any) => !item.option).length != 0){
      teplte = { ...teplte, ...teplte.list };
      delete teplte.list;
    }
    // if(teplte.list && Object.keys(teplte.list).length != 0){
    //   teplte = { ...teplte, ...teplte.list };
    //   delete teplte.list;
    // }
    return teplte;
  }).map((verList: any) => {
    verList.componentList.forEach((item: any) => {
      shi_shi_zhu_zi(item);
      if(item.key == "group"){
        item.groupList.forEach((vv2: any) => {
          shi_shi_zhu_zi(vv2);
          if(vv2.request?.requestParams?.["Params"] && Array.isArray(vv2.request.requestParams["Params"])){
            vv2.request.requestParams["Params"].forEach((qqq: any) => {
              arr.push({ dev_id: qqq.value1, identifier: qqq.value2 });
            })
          }
        })
      }else{
        if(item.request?.requestParams?.["Params"] && Array.isArray(item.request.requestParams["Params"])){
          item.request.requestParams["Params"].forEach((qqq: any) => {
            arr.push({ dev_id: qqq.value1, identifier: qqq.value2 });
          })
        }
      }
    })
    return verList;
  })

  console.log(verTeplte)
  const submittedSignature = editorDocumentSignature(chartEditStore)
  const thumbnail = await captureCanvasThumbnail()

  try {
    const bindingResult: any = await configurationPointBindings({
      user_id: userId.value,
      config_id: wk_Config_id.value,
      bindings: arr
    });
    if(bindingResult.code == 200 && bindingResult.msg == "ok"){
      const res: any = await apiV1Config({
        user_id: userId.value,
        config_id: wk_Config_id.value,
        username: "test",
        ...(thumbnail ? { thumbnail } : {}),
        main_data: { item: Array.isArray(verTeplte) ? verTeplte : [verTeplte] }
      });
      if(res.success){
        markEditorSaved(submittedSignature)
        sessionStorage.removeItem(recoveryDraftKey())
        window['$message'].success(res.message);
        let listlist = JSONParse(sessionStorage.getItem("zhutaineirong") as any);
        if(listlist){
          listlist.main_data.item = Array.isArray(verTeplte) ? verTeplte : [verTeplte];
          sessionStorage.setItem("zhutaineirong", JSONStringify(listlist));
        }
      }
    }else{
      window['$message'].error("点位存储失败，组态整体存储失败！");
    }
  } catch {
    // 请求层已给出具体失败提示；消费异步错误，避免预览自动保存产生未处理异常。
    return false;
  }
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
    title: '保存',
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
