import { ref } from "vue";

/**
 * 触发方式列表
 */
export let optionList1 = ref([
  { value: "满足条件", label: "满足条件" },
  { value: "单击", label: "单击" },
  { value: "双击", label: "双击" },
  { value: "鼠标移入", label: "鼠标移入" },
  { value: "鼠标移出", label: "鼠标移出" },
  { value: "渲染前", label: "渲染前" },
  { value: "渲染后", label: "渲染后" }
]);

/**
 * 执行动作列表
 */
export let optionList2 = ref([
  { value: "更新属性", label: "更新属性" },
  { value: "页面跳转", label: "页面跳转" },
  { value: "打开链接", label: "打开链接" },
  { value: "刷新页面", label: "刷新页面" },
  { value: "刷新数据", label: "刷新数据" },
  { value: "禁启用", label: "禁启用" },
  { value: "显隐切换", label: "显隐切换" }
]);

/**
 * 目标组件
 */
export let optionList3 = ref([]);

/**
 * 目标页面
 */
export let optionList4 = ref([]);

/**
 * 添加条件运算符
 */
export let optionList5 = ref([
    { value: "等于", label: "等于", content: "a=b" },
    { value: "不等于", label: "不等于", content: "a!=b" },
    { value: "小于", label: "小于", content: "a<b" },
    { value: "大于", label: "大于", content: "a>b" },
    { value: "小于等于", label: "小于等于", content: "a<=b" },
    { value: "大于等于", label: "大于等于", content: "a>=b" }
]);

/**
 * 根据value筛选第一项
 */
export let optionListFileter = (option: any,i: number) => {
    return option.find((item: any) => item.value === i).label;
}

/**
 * 更新目标组件列表
 */
export function resetTrget(e: any) {
    optionList3.value = e.map((item: any) => ({
        ...item,
        value: item.id,
        label: item.chartConfig.title
    }))
}

/**
 * 更新目标页面列表
 */
export function resetPage(e: any) {
    optionList4.value = e.map((item: any) => ({
        ...item,
        value: item.title,
        label: item.title
    }))
}

/**
 * 根据组件id数组，筛选出组件对象
 */
export let resetTrgetFilterFun = function(list: Array<any>){
    if(!list) return;
    if(!Array.isArray(list)){
        list = [list];
    }
    let arr: any = [];
    optionList3.value.forEach((c: any) => {
        list.forEach(v => {
            if(c.id == v){
                arr.push(c);
            }
        })
    })
    return arr;
}
