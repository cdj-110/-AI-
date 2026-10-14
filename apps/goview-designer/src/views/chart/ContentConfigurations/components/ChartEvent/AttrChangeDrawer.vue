<template>
  <n-drawer v-model:show="drawerShow" :width="370" placement="right">
    <n-drawer-content>
      <template #header>
        <div style="display: flex;align-items: center;justify-content: space-between;">
          <div>更改属性</div>
          <div>
            <n-button size="tiny" @click="drawerShow = false;">取消</n-button>
            <n-button type="primary" size="tiny" @click="onSave" style="margin-left: 1vw;color: #fff;">保存</n-button>
          </div>
        </div>
      </template>
      <div v-if="localData">
        <n-text depth="3" style="display: block; margin-bottom: 16px; font-size: 12px;">当前修改实时预览，保存后在事件触发时生效。</n-text>
        <!-- 名称 -->
        <name-setting :chartConfig="localData.chartConfig"></name-setting>
        <!-- 尺寸 -->
        <size-setting :isGroup="localData.isGroup" :chartAttr="localData.attr"></size-setting>
        <!-- 位置 -->
        <!-- <position-setting :chartAttr="localData.attr" :canvasConfig="chartEditStore.getEditCanvasConfig" /> -->
        <!-- 滤镜 -->
        <styles-setting :isGroup="localData.isGroup" :chartStyles="localData.styles"></styles-setting>
        <!-- 自定义配置项 -->
        <component :is="localData.chartConfig.conKey" :optionData="localData.option" v-bind="localData.chartConfig.key === 'Figure' ? { chartAttr: localData.attr } : {}"></component>
      </div>
    </n-drawer-content>
  </n-drawer>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { deepClone } from '@/utils/utils'
import { useTargetData } from '../hooks/useTargetData.hook'
import { NameSetting, PositionSetting, SizeSetting, StylesSetting } from '@/components/Pages/ChartItemSetting'
import { clearEventAttributePreview, setEventAttributePreview } from '@/utils/eventAttributePreview'

const props = defineProps<{
  show: boolean,
  eventItem: any
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void,
  (e: 'save', payload: { snapshot: any, eventItem: any }): void
}>()

const { chartEditStore } = useTargetData()

const drawerShow = computed({
  get: () => props.show,
  set: (v) => emit('update:show', v)
})

/**
 * 递归在画布组件列表（含分组 groupList）中按 id 查找目标组件
 */
let findById = (list: any[], id: string): any => {
  for (const item of list) {
    if (item.id === id) return item
    if (item.isGroup && Array.isArray(item.groupList)) {
      const found = findById(item.groupList, id)
      if (found) return found
    }
  }
  return null
}

/**
 * drawer 内部编辑的本地副本（ref 包对象会自动 deep reactive，子组件改属性才能生效）
 */
const localData = ref<any>(null)
const owner = Symbol('event-attribute-drawer')
let previewId = ''
let openedCanvas = ''
const clearPreview = () => {
  if (previewId) clearEventAttributePreview(previewId, owner)
  previewId = ''
}
const targetId = computed(() => {
  const target = props.eventItem?.form?.targetComponent
  return Array.isArray(target) ? target[0] : target
})
const snapshotOf = (data: any) => deepClone({ option: data.option, attr: data.attr, styles: data.styles, chartConfig: data.chartConfig })
const previewOf = (data: any) => ({ option: data.option, attr: data.attr, styles: data.styles, chartConfig: data.chartConfig })

watch([() => props.show, targetId], ([val]) => {
  clearPreview()
  if (val) {
    if (!targetId.value) {
      window['$message']?.warning('请先选择目标组件')
      drawerShow.value = false
      return
    }
    const found = findById(chartEditStore.getComponentList, targetId.value)
    if (!found) {
      window['$message']?.warning('未找到目标组件，请确认目标组件已添加到画布')
      drawerShow.value = false
      return
    }
    localData.value = deepClone(found)
    const saved = props.eventItem?.form?.saveAttrArr
    if (saved && typeof saved === 'object') {
      for (const key of ['option', 'attr', 'styles', 'chartConfig']) {
        if (saved[key] !== undefined) localData.value[key] = deepClone(saved[key])
      }
    }
    previewId = found.id
    openedCanvas = chartEditStore.getCurrentCanvasTitle
    setEventAttributePreview(previewId, owner, previewOf(localData.value))
  } else {
    localData.value = null
  }
}, { immediate: true })

watch(() => localData.value && [localData.value.option, localData.value.attr, localData.value.styles, localData.value.chartConfig], () => {
  if (props.show && previewId && localData.value) setEventAttributePreview(previewId, owner, previewOf(localData.value))
}, { flush: 'sync' })
watch(() => chartEditStore.getCurrentCanvasTitle, title => {
  if (props.show && openedCanvas && title !== openedCanvas) { clearPreview(); drawerShow.value = false }
})
watch(() => findById(chartEditStore.getComponentList, targetId.value), found => {
  if (props.show && previewId && !found) { clearPreview(); drawerShow.value = false }
})
onBeforeUnmount(clearPreview)

/**
 * 保存：把修改后的属性快照发给父组件
 */
const onSave = () => {
  if (!localData.value) return
  const snapshot = snapshotOf(localData.value)
  emit('save', { snapshot, eventItem: props.eventItem })
  clearPreview()
  drawerShow.value = false
}
</script>
