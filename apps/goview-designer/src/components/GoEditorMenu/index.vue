<template>
  <n-button quaternary circle aria-label="返回云组态项目" title="关闭编辑器，返回项目" @click="returnToProjects">
    <template #icon><n-icon><CloseIcon /></n-icon></template>
  </n-button>
</template>
<script setup lang="ts">
import { icon } from '@/plugins'
import { goDialog } from '@/utils'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { editorHasUnsavedChanges, platformProjectsUrl } from '@/utils/editorNavigation'
const { CloseIcon } = icon.ionicons5
const editor = useChartEditStore()
const returnToProjects = () => {
  let destination: string
  try { destination = platformProjectsUrl(sessionStorage.getItem('wk_PlatformOrigin'), sessionStorage.getItem('wk_ApiBaseUrl'), window.location.href) }
  catch (error) { window['$message']?.warning((error as Error).message); return }
  const navigate = () => window.location.assign(destination)
  if (!editorHasUnsavedChanges(editor)) { navigate(); return }
  goDialog({ title: '有未保存的修改', message: '返回项目不会保存当前修改。请取消并先点击“保存”，或选择仍然返回。', positiveText: '仍然返回', negativeText: '继续编辑', onPositiveCallback: navigate })
}
</script>
