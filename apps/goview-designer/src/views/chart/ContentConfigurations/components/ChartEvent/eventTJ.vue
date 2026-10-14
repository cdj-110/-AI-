<template>
  <div>
    <n-drawer v-model:show="active" :width="500">
      <n-drawer-content title="条件编辑" closable>
        <n-form ref="formRef" label-placement="left" label-align="left" :show-feedback="false" :label-width="70" :model="tiaoJianList">
          <n-form-item label="判断类型">
            <n-radio-group v-model:value="tiaoJianList.leixingType" name="leixingType">
              <n-radio value="满足全部条件">满足全部条件</n-radio>
              <n-radio value="满足任意条件">满足任意条件</n-radio>
            </n-radio-group>
          </n-form-item>
          <n-form-item>
            <n-button ghost :render-icon="renderIcon(AddIcon)" @click="showModal = true;" style="width: 100%;margin-top: 1.5rem;">添加条件</n-button>
          </n-form-item>
          <div v-for="(item, index) in tiaoJianList.tiaoList" :key="index" style="display: flex;align-items: center;justify-content: space-between;border: 1px solid #5f5f5f;border-radius: 7px;padding: 0.5rem 1rem;margin-top: 1.5rem;">
            <div style="display: flex; align-items: center">
              <div style="margin-right: 1rem; font-weight: 700; font-size: 0.9rem">{{ item.title }}</div>
              <div v-if="item.tiaojianleixingType === '固定值'">
                <span>{{ item.title1 }}</span>&nbsp;&nbsp;&nbsp;
                <span style="color: #77fac1">{{ item.content1 }}</span>&nbsp;&nbsp;&nbsp;
                <span>{{ item.fu }}</span>&nbsp;&nbsp;&nbsp;
                <span>{{ item.value }}</span>
              </div>
              <div v-if="item.tiaojianleixingType === '组件联动'">
                <span>{{ item.title1 }}</span>&nbsp;&nbsp;&nbsp;
                <span style="color: #77fac1">{{ item.content1 }}</span>&nbsp;&nbsp;&nbsp;
                <span>{{ item.fu }}</span>&nbsp;&nbsp;&nbsp;
                <span>{{ item.title2 }}</span>&nbsp;&nbsp;&nbsp;
                <span style="color: #77fac1">{{ item.content2 }}</span>
              </div>
            </div>
            <div style="display: flex; align-items: center">
              <n-icon size="17" :component="EditIcon" style="margin-right: 0.5rem" @click.stop="" />
              <n-icon size="17" :component="TrashIcon" @click.stop="" />
            </div>
          </div>
        </n-form>
      </n-drawer-content>
    </n-drawer>

    <n-modal v-model:show="showModal" preset="card" title="条件编辑" :style="{ width: '500px'}" content-scrollable :close-on-esc="false" :mask-closable="false">
      <div style="border: 1px solid #5f5f5f;border-radius: 7px;padding: 0.5rem 1rem;margin-top: 1rem;">
        <n-form ref="formRef" label-placement="left" label-align="left" :show-feedback="false" :label-width="70" :model="form">
          <div style="font-size: 17px;font-weight: 700;">{{ form.title }}</div>
          <n-form-item label="类型" style="margin: 0.3rem 0;">
            <n-radio-group v-model:value="form.tiaojianleixingType" name="tiaojianleixingType">
              <n-radio value="固定值">固定值</n-radio>
              <n-radio value="组件联动">组件联动</n-radio>
            </n-radio-group>
          </n-form-item>
          <n-form-item label="条件">
            <n-space :wrap="false">
              <n-input type="text" v-model:value="form.content1" size="tiny" @click="tjTrue = true;tjForm.input = 'title1';" placeholder="选择组件属性" />
              <n-select v-model:value="form.fu" placeholder="选择条件" :options="optionList5" size="tiny" :render-label="renderOption" style="width: 150px;" />
              <n-input type="text" v-if="form.tiaojianleixingType === '固定值'" v-model:value="form.value" size="tiny" placeholder="值" />
              <n-input type="text" v-if="form.tiaojianleixingType === '组件联动'" v-model:value="form.content2" @click="tjTrue = true;tjForm.input = 'title2';" size="tiny" placeholder="选择组件属性" />
            </n-space>
          </n-form-item>
          <n-form-item style="margin-top: 1rem;display: flex;justify-content: right;">
            <n-button @click="showModal = false;" style="margin-right: 1rem;">取消</n-button>
            <n-button type="success" @click="saveEvent">保存</n-button>
          </n-form-item>
        </n-form>
      </div>
    </n-modal>
    <n-modal v-model:show="tjTrue" preset="card" title="选择属性" :style="{ width: '500px'}" content-scrollable :close-on-esc="false" :mask-closable="false">
      <n-form ref="formRef" label-placement="left" label-align="left" :show-feedback="false" :label-width="70" :model="tjForm">
        <n-form-item label="选择组件" style="margin: 0.3rem 0;">
          <n-select v-model:value="tjForm.zj1" :on-update:show="onUpdateShow" :on-update:value="onUpdateValue" placeholder="请选择目标组件" :options="optionList3" />
        </n-form-item>
        <n-form-item label="选择属性" style="margin: 0.3rem 0;">
          <n-select v-model:value="tjForm.zj2" :on-update:show="onUpdateShow" placeholder="请选择目标属性" :options="attrList" />
        </n-form-item>
        <n-form-item style="margin-top: 1rem;display: flex;justify-content: right;">
          <n-button @click="tjTrue = false;" style="margin-right: 1rem;">取消</n-button>
          <n-button type="success" @click="saveZj">保存</n-button>
        </n-form-item>
      </n-form>
    </n-modal>
  </div>
</template>

<script lang="ts" setup>
import { h, Ref, ref, watch } from 'vue';
import { icon } from '@/plugins'
import { renderIcon } from "@/utils/utils";
import { optionList3 ,optionList5, resetTrget } from './event';
import { useTargetData } from '../hooks/useTargetData.hook'

let emit = defineEmits(["tiaoJianChange"]);
let { AddIcon,TrashIcon } = icon.ionicons5;
let { EditIcon } = icon.carbon;
let { chartEditStore } = useTargetData()
let active = ref(false);//执行动作弹框是否打开
let tiaoJianTerget: any = ref({});//保存哪一项事件添加的条件
let showModal = ref(false);//添加/编辑条件弹框
let tjTrue = ref(false);//选择组件/属性弹框
let attrList: Ref<Array<any>> = ref([]);//选择属性

// 条件列表
let tiaoJianList: any = ref({
  leixingType: "满足全部条件",
  tiaoList: []
});
let form = ref({ title: "", tiaojianleixingType: "固定值", id1: "", title1: "", content1: "", fu: "", value: "", title2: "", id2: "", content2: "" });//添加/编辑条件
watch(tiaoJianList,value=>{if(active.value&&tiaoJianTerget.value?.id)tiaoJianTerget.value.tiaoJianList=JSON.parse(JSON.stringify(value))},{deep:true});
let tjForm = ref({ input: "", zj1: "", zj2: "" });//选择组件、属性

/**
 * 保存添加/修改条件
 */
let saveEvent = () => {
  if(!form.value.id1||!form.value.content1||!form.value.fu||(form.value.tiaojianleixingType==='固定值'?form.value.value.trim()==='':!form.value.id2||!form.value.content2)){
    window['$message']?.warning('请完整选择属性、运算符和比较值');return;
  }
  tiaoJianList.value.tiaoList.push(form.value);
  form.value = { title: "", tiaojianleixingType: "固定值", id1: "", title1: "", content1: "", fu: "", value: "", title2: "", id2: "", content2: "" };
  showModal.value = false;
  tiaoJianTerget.value.tiaoJianList = tiaoJianList.value;
  emit("tiaoJianChange",tiaoJianTerget.value);
}

/**
 * 选择组件、属性
 */
let saveZj = () => {
  let arr: any = chartEditStore.getComponentList.find(item => item.id == tjForm.value.zj1);
  if(tjForm.value.input == "title1"){
    form.value.title1 = arr.chartConfig.title;
    form.value.content1 = tjForm.value.zj2;
    form.value.id1 = tjForm.value.zj1;
  }else if(tjForm.value.input == "title2"){
    form.value.title2 = arr.chartConfig.title;
    form.value.content2 = tjForm.value.zj2;
    form.value.id2 = tjForm.value.zj1;
  }
  form.value.title = `事件${tiaoJianList.value.tiaoList.length + 1}`;
  tjForm.value = { input: "", zj1: "", zj2: "" };
  tjTrue.value = false;
}

/**
 * 选择组件之后渲染属性
 */
let onUpdateValue = (e: any) => {
  tjForm.value.zj1 = e;
  let arr: any = chartEditStore.getComponentList.find((item: any) => item.id == e);

  let ver = { ...arr?.attr, ...arr?.option, ...arr?.styles };
  attrList.value = [
    {
      type: "group",
      label: "属性列表",
      key: "属性列表",
      children: Object.keys(ver).map(item => ({
        value: item,
        label: item,
      }))
    },
    {
      type: "group",
      label: "点位信息",
      key: "点位信息",
      children: arr?.request.requestParams.Params && Array.isArray(arr?.request.requestParams.Params) && arr?.request.requestParams.Params.length !=0 ? arr?.request.requestParams.Params.map((item: any) => ({
        value: item.value1,
        label: item.value2,
      })) : []
    }
  ];
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
 * 自定义渲染选择大于、等于下拉框
 */
let renderOption = (option: any) => {
  return h("div", {
    style: {
      display: "flex",
      justifyContent: "space-between",
      alignItems: "center"
    }
  }, [
    h("p",{style: {fontSize: "12px",marginRight: "1rem"}},option.label),
    h("p",{},option.content),
  ])
}

/**
 * 由父组件触发
 */
let eventTJChange = (e: any, type: boolean) => {
  active.value = type;
  tiaoJianTerget.value = e;
  tiaoJianList.value=JSON.parse(JSON.stringify(e.tiaoJianList||{leixingType:'满足全部条件',tiaoList:[]}));
}

defineExpose({ eventTJChange });
</script>

<style lang="scss" scoped></style>
