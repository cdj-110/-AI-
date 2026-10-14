<template>
  <n-modal
    class="go-chart-data-request"
    v-model:show="modelShowRef"
    :mask-closable="false"
    :closeOnEsc="true"
    :onEsc="onEsc"
  >
    <n-card :bordered="false" role="dialog" size="small" aria-modal="true" style="width: min(1080px, calc(100vw - 48px)); height: 720px;">
      <template #header>
        <div class="request-dialog-title">
          <div class="request-dialog-title__main">{{ isHistoryRequest ? '绑定历史数据' : '绑定数据' }}</div>
          <div class="request-dialog-title__desc">选择设备点位并配置组件的数据查询范围</div>
        </div>
      </template>
      <n-scrollbar style="max-height: 602px">
        <div class="go-pr-3">
          <n-space vertical>
            <!-- <request-global-config></request-global-config> -->
            <request-target-config :target-data-request="targetData?.request" :targetList="targetData" :isRuleTrue="isRuleTrue"></request-target-config>
          </n-space>
        </div>
      </n-scrollbar>
      <!-- 底部 -->
      <template #action>
        <n-space justify="space-between">
          <div>
            <n-text>「 {{ chartConfig.categoryName }} 」</n-text>
            <n-text>—— </n-text>
            <n-tag type="primary" :bordered="false" style="border-radius: 5px">
              {{ isHistoryRequest ? '历史数据' : requestContentTypeObj[requestContentType] }}
            </n-tag>
          </div>
          <div>
            <n-button class="go-mr-3" @click="closeHandle">取消</n-button>
            <n-button type="primary" @click="closeAndSendHandle"> {{ saveBtnText || '确认绑定' }}</n-button>
          </div>
        </n-space>
      </template>
    </n-card>
  </n-modal>
</template>

<script lang="ts" setup>
import { ref, toRefs, PropType, watch, computed } from 'vue'
import { RequestContentTypeEnum } from '@/enums/httpEnum'
import { useTargetData } from '../../../hooks/useTargetData.hook'
import { RequestGlobalConfig } from './components/RequestGlobalConfig'
import { RequestTargetConfig } from './components/RequestTargetConfig'
import { CreateComponentType, CreateComponentGroupType } from '@/packages/index.d'

const props = defineProps({
  modelShow: Boolean,
  targetData: Object as PropType<CreateComponentType>,
  saveBtnText: String || null
})
const emit = defineEmits(['update:modelShow', 'sendHandle'])
let isRuleTrue = ref(false);//是否点击了添加属性的按钮 true点击了，false未点击

// 解构基础配置
const { chartConfig } = toRefs(props.targetData as CreateComponentType)
const { requestContentType } = toRefs((props.targetData as CreateComponentType).request)
const modelShowRef = ref(false)
const requestContentTypeObj = {
  [RequestContentTypeEnum.DEFAULT]: '普通请求',
  [RequestContentTypeEnum.SQL]: 'SQL 请求'
}
const isHistoryRequest = computed(() => (props.targetData as CreateComponentType)?.request?.requestHttpType === '历史数据')

watch(
  () => props.modelShow,
  newValue => {
    modelShowRef.value = newValue
  },
  {
    immediate: true
  }
)

const closeHandle = () => {
  emit('update:modelShow', false)
}

const closeAndSendHandle = () => {
  emit('update:modelShow', false)
  emit('sendHandle')
  isRuleTrue.value = true;
}

const onEsc = () => {
  closeHandle()
}
</script>

<style lang="scss" scoped>
@include go('chart-data-request') {
  &.n-card.n-modal,
  .n-card {
    @extend .go-background-filter;
  }
  .n-card-shallow {
    background-color: rgba(0, 0, 0, 0) !important;
  }
  @include deep() {
    & > .n-card__content {
      padding-right: 0;
    }
    .n-card__content {
      padding-bottom: 5px;
    }
  }
}

.request-dialog-title {
  display: flex;
  align-items: baseline;
  gap: 12px;

  &__main {
    font-size: 18px;
    font-weight: 600;
  }

  &__desc {
    color: var(--n-text-color-3);
    font-size: 13px;
    font-weight: 400;
  }
}
</style>
