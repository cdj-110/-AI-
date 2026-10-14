<template>
  <!-- 事件配置 -->
  <n-collapse class="go-mt-3" arrow-placement="right" :default-expanded-names="['1', '2']">
    <n-text depth="3">
      组件 id：
      <n-text>{{ targetData.id }}</n-text>
    </n-text>
    <div style="font-size: 0.9rem;padding: 0.5rem 0;border-top: 1px solid #5f5f5f;margin-top: 1rem;">自定义事件配置</div>
    <n-button ghost :render-icon="renderIcon(AddIcon)" style="width: 100%;" @click="eventChange({},0)">添加事件</n-button>
    
    <div style="margin: 1rem 0;">
      <n-checkbox-group v-model:value="checkList" @update:value="onUpdateValue">
        <div v-for="(item, index) in eventList" :key="index" style="border: 1px solid #5f5f5f;border-radius: 7px;padding: 0.5rem 1rem;margin: 1rem 0;">
          <n-checkbox v-if="item.isNewly == false" :value="item.id" style="width: 100%;">
            <template #default>
              <div style="display: flex;align-items: center;justify-content: space-between;">
                <div style="display: flex;align-items: center;color: #BCBCBC;">
                  <n-icon size="17" :component="AccountIcon" style="margin-right: 0.3rem;" />
                  <span style="color: #77FAC1;">{{ optionListFileter(optionList1,item.changeType) }}</span>{{ item.title }}</div>
                <div>
                  <n-icon size="17" :component="EditIcon" style="margin-right: 0.5rem;" @click.stop="eventChange(item,1)" />
                  <n-icon size="17" :component="TrashIcon" @click.stop="eventChange(item,2)" />
                </div>
              </div>
            </template>
          </n-checkbox>
          <div v-if="item.isNewly == false" style="padding: 0.5rem 0;color: #BCBCBC;">
            <span style="color: #77FAC1;">{{ optionListFileter(optionList1,item.changeType) }}</span> 时 <span style="color: #77FAC1;">{{ optionListFileter(optionList2,item.changeAttr) }}</span>
          </div>
          <div v-if="item.isNewly == false" style="color: #BCBCBC;">{{ item.targetComponent }}</div>
          <condition-summary v-if="!item.isNewly" :group="item.tiaoJianList" :components="conditionComponents" style="margin-top:8px" />
          <div style="color: #BCBCBC !important;" v-if="item.isNewly">
            <div v-if="item.isNewly == false" style="background: #5f5f5f;margin: 0.5rem 0;height: 1px;"></div>
            <n-form ref="formRef" label-placement="left" label-align="left" :label-width="70" :model="item.form">
              <n-form-item label="配置模式">
                <n-radio-group v-model:value="item.form.peizhiType" name="peizhiType">
                  <n-radio value="表单配置">表单配置</n-radio>
                  <n-radio value="代码编写">代码编写</n-radio>
                </n-radio-group>
              </n-form-item>
              <n-form-item label="触发方式">
                <n-select v-model:value="item.form.changeType" placeholder="请选择触发方式" :options="optionList1" size="tiny" />
              </n-form-item>
              <n-form-item label="条件">
                <div style="width:100%;min-width:0">
                  <condition-summary :group="item.tiaoJianList" :components="conditionComponents" style="margin-bottom:8px" />
                  <n-button ghost :render-icon="renderIcon(AddIcon)" @click="eventTJChange(item,index)" size="tiny" style="width: 100%;">添加条件</n-button>
                  <eventTJ ref="eventTJRef"></eventTJ>
                </div>
              </n-form-item>
              <n-text v-if="item.form.changeType==='满足条件'" depth="3" style="display:block;font-size:12px;margin-bottom:12px">预览/运行时自动判断。首次满足或从不满足变为满足时执行一次，持续满足不会重复执行；未配置条件不触发。</n-text>
              <n-form-item label="执行动作">
                  <n-select v-model:value="item.form.changeAttr" @update:value="changeAttrChange(item)" placeholder="请选择执行动作" :options="optionList2" size="tiny" />
              </n-form-item>
              <n-form-item label="禁启用" v-if="item.form.changeAttr == '禁启用'">
                <n-tabs type="segment" size="small" :value="item.form.startTrue" :on-update:value="((e: any) => item.form.startTrue = e)" animated>
                  <n-tab-pane name="启用" tab="启用"></n-tab-pane>
                  <n-tab-pane name="禁止" tab="禁止"></n-tab-pane>
                  <n-tab-pane name="切换" tab="切换"></n-tab-pane>
                </n-tabs>
              </n-form-item>
              <n-form-item label="显隐" v-if="item.form.changeAttr == '显隐切换'">
                <n-tabs type="segment" size="small" :value="item.form.xianTrue" :on-update:value="((e: any) => item.form.xianTrue = e)" animated>
                  <n-tab-pane name="显示" tab="显示"></n-tab-pane>
                  <n-tab-pane name="隐藏" tab="隐藏"></n-tab-pane>
                  <n-tab-pane name="切换" tab="切换"></n-tab-pane>
                </n-tabs>
              </n-form-item>
              <n-form-item label="筛选条件" v-if="item.form.changeAttr == '刷新数据'">
                <n-select v-model:value="item.form.searchValue" :on-update:show="onUpdateShow" placeholder="请选择筛选条件" :options="optionList3" multiple size="tiny" />
              </n-form-item>
              <n-form-item label="目标组件" v-if="item.form.changeAttr == '更新属性' || item.form.changeAttr == '刷新数据' || item.form.changeAttr == '禁启用' || item.form.changeAttr == '显隐切换'">
                  <n-select v-model:value="item.form.targetComponent" :on-update:show="onUpdateShow" placeholder="请选择目标组件" :options="optionList3" :multiple="item.form.changeAttr !== '更新属性'" size="tiny" />
              </n-form-item>
              <n-form-item label=" " v-if="item.form.changeAttr === '更新属性'" style="width: 100%;">
                <n-button style="width: 100%;" @click="isAttrActiveTrue(item)">编辑组件属性</n-button>
              </n-form-item>
              <n-form-item label="目标页面" v-if="item.form.changeAttr == '页面跳转'">
                  <n-select v-model:value="item.form.pageValue" :on-update:show="onpageValueShow" placeholder="请选择目标页面" :options="optionList4" size="tiny" />
              </n-form-item>
              <n-form-item label="打开方式" v-if="item.form.changeAttr == '打开链接'">
                <n-tabs type="segment" size="small" :value="item.form.startType" :on-update:value="((e: any) => item.form.startType = e)" animated>
                  <n-tab-pane name="当前窗口" tab="当前窗口"></n-tab-pane>
                  <n-tab-pane name="新窗口" tab="新窗口"></n-tab-pane>
                </n-tabs>
              </n-form-item>
              <n-form-item label="链接地址" v-if="item.form.changeAttr == '打开链接'">
                <n-input type="text" v-model:value="item.form.urlLink" size="tiny" placeholder="请输入链接地址" />
              </n-form-item>
              <n-form-item style="margin-top: 1rem;display: flex;justify-content: right;">
                <n-button type="success" @click="saveEvent">保存</n-button>
              </n-form-item>
            </n-form>
          </div>
        </div>
      </n-checkbox-group>
    </div>
    <attr-change-drawer v-model:show="activeTrue" :event-item="attrEditTarget" @save="saveAttrSnapshot" />
  </n-collapse>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { renderIcon, getUUID, deepClone, JSONParse, JSONStringify } from "@/utils/utils";
import { icon } from '@/plugins'
import { ChartEventInteraction } from './components/ChartEventInteraction'
import { ChartEventAdvancedHandle } from './components/ChartEventAdvancedHandle'
import { ChartEventBaseHandle } from './components/ChartEventBaseHandle'
import { useTargetData } from '../hooks/useTargetData.hook'
import eventTJ from './eventTJ.vue';
import AttrChangeDrawer from './AttrChangeDrawer.vue';
import ConditionSummary from './ConditionSummary.vue';
import { flattenEventComponents } from '@/utils/conditionEvents';
import { optionList1, optionList2, optionList3, optionList4, optionListFileter, resetTrget, resetPage, resetTrgetFilterFun } from './event';
import { NameSetting, PositionSetting, SizeSetting, StylesSetting } from '@/components/Pages/ChartItemSetting'

const { AddIcon,TrashIcon } = icon.ionicons5;
const { AccountIcon,EditIcon } = icon.carbon;

const { targetData, chartEditStore } = useTargetData()
const conditionComponents = computed(() => flattenEventComponents(chartEditStore.getComponentList))

let activeTrue = ref(false);
const attrEditTarget = ref<any>(null);//当前正在编辑属性的事件对象（传给弹窗）
const saveAttrSnapshot = (payload: any) => {
  if (attrEditTarget.value?.form) attrEditTarget.value.form.saveAttrArr = payload.snapshot
}

const createEventDraft = () => ({
  id: "",
  title: "",
  changeType: "",
  changeAttr: "",
  targetComponent: "",
  targetList: [],
  atProp: {},
  isNewly: false,//是否新增
  newly: 0,//0新增 1编辑
  form: {
    peizhiType: "表单配置",
    changeType: "",
    changeAttr: "",
    targetComponent: [],
    startTrue: "启用",
    xianTrue: "显示",
    pageValue: "",
    startType: "当前窗口",
    urlLink: "",
    searchValue: "",
    saveAttrArr: {}
  }
})

let eventVirtual = ref(createEventDraft());

let eventList: any = ref([]);
let checkList = ref([]);//选中的事件列表
let eventTJRef: any = ref("");//添加条件的弹框Ref
let saveAttrList = ref<any>({});//暂存未更改前的属性数据
let saveAttrArr = ref({});//所有修改完的属性数据

/**
 * 将事件写回当前组件、当前画布缓存及多画布列表。
 * 事件面板以组件数据为唯一来源，避免旧的 sessionStorage 快照覆盖刚保存的事件。
 */
const persistEventList = () => {
  const savedEvents = deepClone(eventList.value.filter((item: any) => item.id && !item.isNewly))
  targetData.value.interactActions = savedEvents

  const canvasTitle = chartEditStore.getCurrentCanvasTitle || JSONParse(sessionStorage.getItem("chunyixiazhutai") as any)?.title
  const storageInfo = deepClone(chartEditStore.getStorageInfo())
  const pageIndex = chartEditStore.pageCallList.findIndex((page: any) => page.title === canvasTitle)
  if (pageIndex !== -1) {
    const page = chartEditStore.pageCallList[pageIndex]
    Object.assign(page, storageInfo)
    if (page.list) page.list = deepClone(storageInfo)
  }

  sessionStorage.setItem("chunyixiazhutai", JSONStringify({ title: canvasTitle, ...storageInfo }))
}

/**
 * 显示出事件的具体内容
 * @inner(e) 如果传入具体的事件，就显示具体的事件，否则就显示读取到的事件
 */
let getList = (e?: Array<any>) => {
  const actions = e && Array.isArray(e) ? e : targetData.value?.interactActions
  eventList.value = Array.isArray(actions) ? deepClone(actions) : []
}

watch(() => targetData.value?.id, () => {
  getList();
},{immediate: true});

/**
 * 新增0/编辑1/删除2事件
 */
let eventChange = (list: any,type: any) => {
  switch(type) {
    case 0:
      eventVirtual.value = createEventDraft();
      eventVirtual.value.newly = 0;
      eventVirtual.value.isNewly = true;
      eventVirtual.value.id = getUUID(20,"eventid_");
      eventList.value.push(eventVirtual.value);
      break;
    case 1:
      eventList.value.forEach((item: any) => {
        if(item.id == list.id){
          item.isNewly = true;
          item.newly = 1;
          eventVirtual.value = item;
        }
      })
      break;
    case 2:
      eventList.value = eventList.value.filter((item: any) => item.id != list.id);
      persistEventList();
  }
}

/**
 * 保存事件      
 */
let saveEvent = () => {
  if (!eventVirtual.value.form.changeType || !eventVirtual.value.form.changeAttr) {
    window['$message']?.warning('请选择触发方式和执行动作');
    return;
  }
  if (eventVirtual.value.form.changeAttr === '页面跳转' && !eventVirtual.value.form.pageValue) {
    window['$message']?.warning('请选择目标页面');
    return;
  }
  if(eventVirtual.value.form.changeType==='满足条件'&&!eventVirtual.value.tiaoJianList?.tiaoList?.length){
    window['$message']?.warning('请先添加至少一个有效条件');return;
  }
  eventList.value = eventList.value.map((item: any) => {
    eventVirtual.value.isNewly = false;
    eventVirtual.value.title = `事件${eventList.value.length}`;
    // 事件中只保留来源组件标识。保存整个组件会把 interactActions 再次嵌套进去，
    // 每保存一次体积都会成倍增长，并可能导致预览解析迟缓或失败。
    eventVirtual.value.atProp = {
      id: targetData.value.id,
      key: targetData.value.key,
      chartConfig: targetData.value.chartConfig
        ? { key: targetData.value.chartConfig.key, title: targetData.value.chartConfig.title }
        : undefined
    };
    eventVirtual.value.targetList = deepClone(resetTrgetFilterFun(eventVirtual.value.form.targetComponent));
    if(item.id == eventVirtual.value.id){
      item = Object.assign({}, eventVirtual.value, eventVirtual.value.form);
      item.targetComponent = eventVirtual.value.targetList.map((item: any) => item.chartConfig.title).join(",");
    }
    return item;
  })
  eventVirtual.value = createEventDraft();
  eventList.value = eventList.value.filter((item: any) => item.id);
  persistEventList();
  window['$message']?.success('事件已保存');
}

/**
 * 点击编辑属性按钮
 */
let isAttrActiveTrue = (e: any) => {
  attrEditTarget.value = e;
  activeTrue.value = true;
  saveAttrList.value = e.targetList[0];
}

/**
 * 更改属性
 */
let saveAttrChange = (list: any) => {
}

/**
 * 目标组件下拉框更新里面的组件
 */
let onUpdateShow = (e:boolean) => {
  if(e){
    resetTrget(chartEditStore.getComponentList);
  }
}

/**
 * 目标页面下拉框
 */
let onpageValueShow = (e:boolean) => {
  if(e){
    resetPage(chartEditStore.getPageCallList);
  }
}

/**
 * 执行动作下拉框值改变时触发
 */
let changeAttrChange = (e: any) => {
  e.form.targetComponent = [];
}

/**
 * 获取选中的事件
 */
let onUpdateValue = (e: any) => {
  checkList.value = e;
}

/**
 * 点击添加条件按钮打开添加条件的弹框
 */
let eventTJChange = (e: any,index: any) => {
  eventTJRef.value.length < 1 ? eventTJRef.value[index].eventTJChange(e,true) : eventTJRef.value[0].eventTJChange(e,true);
}
</script>

<style lang="scss" scoped>
:deep(.n-checkbox .n-checkbox__label){
  width: 100%;
}
</style>
