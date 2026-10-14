import { PublicConfigClass } from '@/packages/public'
import { CreateComponentType } from '@/packages/index.d'
import { InputFun } from './index'
import cloneDeep from 'lodash/cloneDeep'
import { chartInitConfig } from '@/settings/designSetting'

export const option = {
    //这是组件属性的传值通道
    borderColor: "#3774B6ff",
    placeholder: "请输入内容", // 默认占位提示
    type: "text",             // 默认类型：文本
    prefix: "",               // 默认前缀：空
    suffix: "",               // 默认后缀：空
    size: "medium",           // 默认尺寸：中
    minLength: 0,             // 默认最小长度：0（无限制）
    maxLength: 1,             // 默认最大长度：0（无限制）
    enableRadius: false,      // 默认不启用圆角
    showCount: false, // 新增：显示数量默认关闭
    value:''
}

export default class Config extends PublicConfigClass implements CreateComponentType {
    public key = InputFun.key
    public attr = { ...chartInitConfig, w: 300, h: 30, zIndex: -1 } // 按钮的高度-后来加的  不要可以去掉
    public chartConfig = cloneDeep(InputFun)
    public option = cloneDeep(option)
}
