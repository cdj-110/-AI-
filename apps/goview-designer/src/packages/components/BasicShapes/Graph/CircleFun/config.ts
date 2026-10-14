import { PublicConfigClass } from '@/packages/public'
import { CreateComponentType } from '@/packages/index.d'
import { CircleFun } from './index'
import cloneDeep from 'lodash/cloneDeep'
import { chartInitConfig } from '@/settings/designSetting'
export const option = {
    //这是组件属性的传值通道
    borderColor: "#3774B6ff",
    backgroundColor: '#163D50',
    gradientColor: ["#163D50", "#163D50"],
    backgroundLinearNumber: 70,
    fillOpacity: 1,
    strokeOpacity: 1,
    colorIsLine: true,
    // 新增：边框虚线配置
    borderType: 'solid',        // 'solid' 或 'dashed'
    dashLength: 5,              // 虚线线段长度（px）
    gapLength: 1,               // 虚线间隙长度（px）
    borderWidth: 3,             // 边框宽度（px）
    showBorder: false,
}

export default class Config extends PublicConfigClass implements CreateComponentType {
    public key = CircleFun.key
    public attr = { ...chartInitConfig, w: 200, h: 200, zIndex: -1 } // 按钮的高度-后来加的  不要可以去掉
    public chartConfig = cloneDeep(CircleFun)
    public option = cloneDeep(option)
}
