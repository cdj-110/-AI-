<template>
  <div class="go-canvas-setting">
    <n-form inline :label-width="45" size="small" label-placement="left">
      <n-form-item label="宽度">
        <!-- 尺寸选择 -->
        <n-input-number
          size="small"
          v-model:value="canvasConfig.width"
          :disabled="editCanvas.lockScale"
          :validator="validator"
          @update:value="changeSizeHandle"
        ></n-input-number>
      </n-form-item>
      <n-form-item label="高度">
        <n-input-number
          size="small"
          v-model:value="canvasConfig.height"
          :disabled="editCanvas.lockScale"
          :validator="validator"
          @update:value="changeSizeHandle"
        ></n-input-number>
      </n-form-item>
    </n-form>

    <div class="upload-box">
      <n-upload
        v-model:file-list="uploadFileListRef"
        :show-file-list="false"
        :customRequest="customRequest"
        :onBeforeUpload="beforeUploadHandle"
      >
        <n-upload-dragger>
          <img v-if="canvasConfig.backgroundImage" class="upload-show" :src="canvasConfig.backgroundImage" alt="背景" />
          <div class="upload-img" v-show="!canvasConfig.backgroundImage">
            <img src="@/assets/images/canvas/noImage.png" />
            <n-text class="upload-desc" depth="3">
              背景图需小于 {{ backgroundImageSize }}M ，格式为 png/jpg/gif 的文件
            </n-text>
          </div>
        </n-upload-dragger>
      </n-upload>
    </div>
    <n-space vertical :size="12">
      <n-space>
        <n-text>背景颜色</n-text>
        <div class="picker-height">
          <n-color-picker
            v-if="!switchSelectColorLoading"
            size="small"
            style="width: 250px"
            v-model:value="canvasConfig.background"
            :showPreview="true"
            :swatches="swatchesColors"
          ></n-color-picker>
        </div>
      </n-space>
      <n-space>
        <n-text>应用类型</n-text>
        <n-select
          size="small"
          style="width: 250px"
          v-model:value="selectColorValue"
          :disabled="!canvasConfig.backgroundImage"
          :options="selectColorOptions"
          @update:value="selectColorValueHandle"
        />
      </n-space>
      <n-space>
        <n-text>背景控制</n-text>
        <n-button class="clear-btn" size="small" :disabled="!canvasConfig.backgroundImage" @click="clearImage">
          清除背景
        </n-button>
        <n-button class="clear-btn" size="small" :disabled="!canvasConfig.background" @click="clearColor">
          清除颜色
        </n-button>
      </n-space>
      <n-space>
        <n-text>适配方式</n-text>
        <n-button-group>
          <n-button
            v-for="item in previewTypeList"
            :key="item.key"
            :type="canvasConfig.previewScaleType === item.key ? 'primary' : 'tertiary'"
            ghost
            size="small"
            @click="selectPreviewType(item.key)"
          >
            <n-tooltip :show-arrow="false" trigger="hover">
              <template #trigger>
                <n-icon class="select-preview-icon" size="18">
                  <component :is="item.icon"></component>
                </n-icon>
              </template>
              {{ item.desc }}
            </n-tooltip>
          </n-button>
        </n-button-group>
      </n-space>

      <n-space :size="3" justify="space-between" style="margin-top: 10px;">
        <n-button
          secondary
          v-for="item in positionList"
          :key="item.key"
          :disabled="item.disabled"
          @click="positonHandle(item.key)"
        >
          <template #icon>
            <n-popover trigger="hover">
              <template #trigger>
                <component :is="item.icon" ></component>
              </template>
              <span>{{ item.lable }}</span>
            </n-popover>
          </template>
        </n-button>
      </n-space>
    </n-space>

    <!-- 滤镜 -->
    <styles-setting :isCanvas="true" :chartStyles="canvasConfig"></styles-setting>
    <n-divider style="margin: 10px 0"></n-divider>

    <!-- 主题选择和全局配置 -->
    <n-tabs class="tabs-box" size="small" type="segment">
      <n-tab-pane
        v-for="item in globalTabList"
        :key="item.key"
        :name="item.key"
        size="small"
        display-directive="show:lazy"
      >
        <template #tab>
          <n-space>
            <span>{{ item.title }}</span>
            <n-icon size="16" class="icon-position">
              <component :is="item.icon"></component>
            </n-icon>
          </n-space>
        </template>
        <component :is="item.render"></component>
      </n-tab-pane>
    </n-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick, watch, onMounted } from 'vue'
import { backgroundImageSize } from '@/settings/designSetting'
import { swatchesColors } from '@/settings/chartThemes/index'
import { FileTypeEnum } from '@/enums/fileTypeEnum'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { EditCanvasConfigEnum } from '@/store/modules/chartEditStore/chartEditStore.d'
import { StylesSetting, PositionSetting } from '@/components/Pages/ChartItemSetting'
import { UploadCustomRequestOptions } from 'naive-ui'
import { JSONParse, fileToUrl, fileTobase64, loadAsyncComponent, renderIcon } from '@/utils'
import { PreviewScaleEnum } from '@/enums/styleEnum'
import { icon } from '@/plugins'
import { apiV1MediaImages } from "@/api/index";
import { baseurlAddress } from "@/utils/request";

const { ColorPaletteIcon } = icon.ionicons5
const {
  ScaleIcon,
  FitToScreenIcon,
  FitToHeightIcon,
  FitToWidthIcon,
  AlignHorizontalLeftIcon,
  AlignVerticalCenterIcon,
  AlignVerticalTopIcon,
  AlignHorizontalCenterIcon,
  AlignHorizontalRightIcon,
  AlignVerticalBottomIcon,
  ColumnIcon,
  RowIcon
} = icon.carbon

const chartEditStore = useChartEditStore()
const canvasConfig = chartEditStore.getEditCanvasConfig
const editCanvas = chartEditStore.getEditCanvas

const uploadFileListRef = ref()
const switchSelectColorLoading = ref(false)
const selectColorValue = ref(0)

const ChartThemeColor = loadAsyncComponent(() => import('./components/ChartThemeColor/index.vue'))
const VChartThemeColor = loadAsyncComponent(() => import('./components/VChartThemeColor/index.vue'))

// 默认应用类型
const selectColorOptions = [
  {
    label: '应用颜色',
    value: 0
  },
  {
    label: '应用背景',
    value: 1
  }
]

const globalTabList = [
  {
    key: 'ChartTheme',
    title: '默认主题',
    icon: ColorPaletteIcon,
    render: ChartThemeColor
  },
  {
    key: 'VChartTheme',
    title: 'VChart主题',
    icon: ColorPaletteIcon,
    render: VChartThemeColor
  },
]

const previewTypeList = [
  {
    key: PreviewScaleEnum.FIT,
    title: '自适应',
    icon: ScaleIcon,
    desc: '自适应比例展示，页面会有留白'
  },
  {
    key: PreviewScaleEnum.SCROLL_Y,
    title: 'Y轴滚动',
    icon: FitToWidthIcon,
    desc: 'X轴铺满，Y轴自适应滚动'
  },
  {
    key: PreviewScaleEnum.SCROLL_X,
    title: 'X轴滚动',
    icon: FitToHeightIcon,
    desc: 'Y轴铺满，X轴自适应滚动'
  },
  {
    key: PreviewScaleEnum.FULL,
    title: '铺满',
    icon: FitToScreenIcon,
    desc: '强行拉伸画面，填充所有视图'
  }
]

let positionList = ref([
  {
    key: 'AlignHorizontalLeftIcon',
    lable: '左对齐',
    disabled: false,
    icon: renderIcon(AlignHorizontalLeftIcon)
  },
  {
    key: 'AlignVerticalCenterIcon',
    lable: '居中对齐（水平）',
    disabled: false,
    icon: renderIcon(AlignVerticalCenterIcon)
  },
  {
    key: 'AlignHorizontalRightIcon',
    lable: '右对齐',
    disabled: false,
    icon: renderIcon(AlignHorizontalRightIcon)
  },
  {
    key: 'AlignVerticalTopIcon',
    lable: '顶部对齐',
    disabled: false,
    icon: renderIcon(AlignVerticalTopIcon)
  },
  {
    key: 'AlignHorizontalCenterIcon',
    lable: '居中对齐（垂直）',
    disabled: false,
    icon: renderIcon(AlignHorizontalCenterIcon)
  },
  {
    key: 'AlignVerticalBottomIcon',
    lable: '底部对齐',
    disabled: false,
    icon: renderIcon(AlignVerticalBottomIcon)
  },
  {
    key: 'RowIcon',
    lable: '垂直分布',
    disabled: false,
    icon: renderIcon(RowIcon)
  },
  {
    key: 'ColumnIcon',
    lable: '水平分布',
    disabled: false,
    icon: renderIcon(ColumnIcon)
  }
])

watch(() => chartEditStore.getTargetChart.selectId,(newval) => {
  if(newval && Array.isArray(newval) && newval.length >= 3){
    positionList.value = positionList.value.map(item => {
      item.disabled = false;
      return item;
    })
  }else{
    positionList.value = positionList.value.map(item => {
      if(item.key == "RowIcon" || item.key == "ColumnIcon"){
        item.disabled = true;
      }
      return item;
    })
  }
},{immediate:true,deep:true})

let chuizhiFunction = (list: any[]) => {
  if (list.length < 3){
    return false;
  }
  let items = [...list].sort((a, b) => a.attr.x - b.attr.x)
  let left = items[0].attr.x
  let right = Math.max(...items.map(i => i.attr.x + i.attr.w))
  let totalWidth = items.reduce((sum, item) => {
    return sum + item.attr.w
  }, 0)
  let gap = (right - left - totalWidth) / (items.length - 1)
  let currentX = left
  items.forEach(item => {
    item.attr.x = currentX
    currentX += item.attr.w + gap
  })
}

let shuipingFunction = (list: any[]) => {
  if (list.length < 3){
    return false;
  }
  let items = [...list].sort((a, b) => a.attr.y - b.attr.y)
  let top = items[0].attr.y
  let bottom = Math.max(...items.map(i => i.attr.y + i.attr.h))
  let totalHeight = items.reduce((sum, item) => {
    return sum + item.attr.h
  }, 0)
  let gap = (bottom - top - totalHeight) / (items.length - 1)
  let currentY = top
  items.forEach(item => {
    item.attr.y = currentY
    currentY += item.attr.h + gap
  })
}

let positonHandle = (key: string) => {
  let keyList = chartEditStore.getComponentList.filter(item => chartEditStore.getTargetChart.selectId.includes(item.id))
  if (keyList.length < 2) return
  let left = Math.min(...keyList.map(i => i.attr.x))
  let right = Math.max(...keyList.map(i => i.attr.x + i.attr.w))
  let top = Math.min(...keyList.map(i => i.attr.y))
  let bottom = Math.max(...keyList.map(i => i.attr.y + i.attr.h))

  switch (key) {
    // 左对齐
    case positionList.value[0].key:
      keyList.forEach(i => {
        i.attr.x = left
      })
      break;

    // X轴居中
    case positionList.value[1].key: {
      const centerY = (top + bottom) / 2
      keyList.forEach(i => {
        i.attr.y = centerY - i.attr.h / 2
      })
      break;
    }

    // 右对齐
    case positionList.value[2].key:
      keyList.forEach(i => {
        i.attr.x = right - i.attr.w
      })
      break;

    // 顶部对齐
    case positionList.value[3].key:
      keyList.forEach(i => {
        i.attr.y = top
      })
      break;

    // Y轴居中
    case positionList.value[4].key: {
      const centerX = (left + right) / 2
      keyList.forEach(i => {
        i.attr.x = centerX - i.attr.w / 2
      })
      break;
    }

    // 底部对齐
    case positionList.value[5].key:
      keyList.forEach(i => {
        i.attr.y = bottom - i.attr.h
      })
      break;

    // 水平分布
    case positionList.value[6].key:
      shuipingFunction(keyList)
      break;

    // 垂直分布
    case positionList.value[7].key:
      chuizhiFunction(keyList)
      break;
  }
}

watch(
  () => canvasConfig.selectColor,
  newValue => {
    selectColorValue.value = newValue ? 0 : 1
  },
  {
    immediate: true
  }
)

// 画布尺寸规则
const validator = (x: number) => x > 50

// 修改尺寸
const changeSizeHandle = () => {
  chartEditStore.computedScale()
}

// 上传图片前置处理
//@ts-ignore
const beforeUploadHandle = async ({ file }) => {
  uploadFileListRef.value = []
  const type = file.file.type
  const size = file.file.size

  if (size > 1024 * 1024 * backgroundImageSize) {
    window['$message'].warning(`图片超出 ${backgroundImageSize}M 限制，请重新上传！`)
    return false
  }
  if (type !== FileTypeEnum.PNG && type !== FileTypeEnum.JPEG && type !== FileTypeEnum.GIF) {
    window['$message'].warning('文件格式不符合，请重新上传！')
    return false
  }
  return true
}

// 应用颜色
const selectColorValueHandle = (value: number) => {
  canvasConfig.selectColor = value == 0
}

// 清除背景
const clearImage = () => {
  chartEditStore.setEditCanvasConfig(EditCanvasConfigEnum.BACKGROUND_IMAGE, undefined)
  chartEditStore.setEditCanvasConfig(EditCanvasConfigEnum.SELECT_COLOR, true)
}

// 启用/关闭 颜色（强制更新）
const switchSelectColorHandle = () => {
  switchSelectColorLoading.value = true
  setTimeout(() => {
    switchSelectColorLoading.value = false
  })
}

// 清除颜色
const clearColor = () => {
  chartEditStore.setEditCanvasConfig(EditCanvasConfigEnum.BACKGROUND, undefined)
  if (canvasConfig.backgroundImage) {
    chartEditStore.setEditCanvasConfig(EditCanvasConfigEnum.SELECT_COLOR, false)
  }
  switchSelectColorHandle()
}

// 自定义上传操作
const customRequest = (options: UploadCustomRequestOptions) => {
  const { file } = options
  nextTick(() => {
    if (file.file) {
      let userId = ref(sessionStorage.getItem('wk_UserId'));
      let wk_Config_id = ref(sessionStorage.getItem('wk_Config_id') as any);
      let chunyixiazhutai = ref(sessionStorage.getItem('chunyixiazhutai') as any);
      let fileList = new FormData();
      fileList.append("file", file.file);
      apiV1MediaImages(fileList, { user_id: userId.value, is_background: 1, config_id: wk_Config_id.value, canvas_name: JSONParse(chunyixiazhutai.value).title }).then(async (res: any) => {
        if(res.url){
          chartEditStore.setEditCanvasConfig(EditCanvasConfigEnum.BACKGROUND_IMAGE, baseurlAddress.urlfile + res.url)
          chartEditStore.setEditCanvasConfig(EditCanvasConfigEnum.SELECT_COLOR, false)
        }else{
          window['$message'].warning("背景图上传出错！");
        }
      })
      // fileTobase64(file.file,((ImageUrl: any) => {
      //   console.log(ImageUrl)
      //   chartEditStore.setEditCanvasConfig(EditCanvasConfigEnum.BACKGROUND_IMAGE, ImageUrl)
      //   chartEditStore.setEditCanvasConfig(EditCanvasConfigEnum.SELECT_COLOR, false)
      // }))
    } else {
      window['$message'].error('添加图片失败，请稍后重试！')
    }
  })
}

// 选择适配方式
const selectPreviewType = (key: PreviewScaleEnum) => {
  chartEditStore.setEditCanvasConfig(EditCanvasConfigEnum.PREVIEW_SCALE_TYPE, key)
}
</script>

<style lang="scss" scoped>
$uploadWidth: 326px;
$uploadHeight: 193px;
@include go(canvas-setting) {
  padding-top: 20px;
  .upload-box {
    cursor: pointer;
    margin-bottom: 20px;
    @include deep() {
      .n-upload-dragger {
        padding: 5px;
        width: $uploadWidth;
        background-color: rgba(0, 0, 0, 0);
      }
    }
    .upload-show {
      width: -webkit-fill-available;
      height: $uploadHeight;
      border-radius: 5px;
    }
    .upload-img {
      display: flex;
      flex-direction: column;
      align-items: center;
      img {
        height: 150px;
      }
      .upload-desc {
        padding: 10px 0;
      }
    }
  }
  .icon-position {
    padding-top: 2px;
  }
  .picker-height {
    min-height: 35px;
  }
  .clear-btn {
    padding-left: 2.25em;
    padding-right: 2.25em;
  }
  .select-preview-icon {
    padding-right: 0.68em;
    padding-left: 0.68em;
  }
  .tabs-box {
    margin-top: 20px;
  }
  .n-button{
    padding: 0 7px;
  }
}
</style>
