import {  PublicConfigClass } from '@/packages/public'
import { CreateComponentType } from '@/packages/index.d'
import { ButtonFun } from './index'
import cloneDeep from 'lodash/cloneDeep'
import { chartInitConfig } from '@/settings/designSetting'

export const option = {
  //这是组件属性的传值通道
  borderColor: "#3774B6ff",
  fontSize: 14,
  fontColor:"#fff",
  bgColor:"#163D50",
    // 边框相关（新增）
  showBorder: false,  // 默认不显示边框
  borderStyle: 'solid',  // 边框样式
  borderWidth: 1,  // 边框宽度
  borderRadius: 4,  // 圆角半径
  fontText: "按钮", //按钮文字
}

export default class Config extends PublicConfigClass implements CreateComponentType {
  public key = ButtonFun.key
  public attr = { ...chartInitConfig, w: 100, h: 50, zIndex: -1 } // 按钮的高度-后来加的  不要可以去掉
  public chartConfig = cloneDeep(ButtonFun)
  public option = cloneDeep(option)
}
