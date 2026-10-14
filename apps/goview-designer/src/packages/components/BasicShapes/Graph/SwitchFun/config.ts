import { PublicConfigClass } from '@/packages/public'
import { CreateComponentType } from '@/packages/index.d'
import { SwitchFun } from './index'
import cloneDeep from 'lodash/cloneDeep'
import { chartInitConfig } from '@/settings/designSetting'
export const option = {
    //这是组件属性的传值通道
    dataset: 0,
    kaiColor: "rgba(255, 255, 255, .20)",
    guanColor: "#3c7eff",
    valueType: 0,
    valueMode: 'auto',
    manualValueType: 'NUMBER',
    controlOnBool: true,
    controlOffBool: false,
    controlEnabled: false,
    controlOnNumber: 1,
    controlOffNumber: 0,
    controlOnText: 'ON',
    controlOffText: 'OFF',
}

export default class Config extends PublicConfigClass implements CreateComponentType {
    public key = SwitchFun.key
    public attr = { ...chartInitConfig, w: 150, h: 60, zIndex: -1 } // 按钮的高度-后来加的  不要可以去掉
    public chartConfig = cloneDeep(SwitchFun)
    public option = cloneDeep(option)
}
