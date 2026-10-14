import {  PublicConfigClass } from '@/packages/public'
import { CreateComponentType } from '@/packages/index.d'
import { Polygon } from './index'
import cloneDeep from 'lodash/cloneDeep'
import { chartInitConfig } from '@/settings/designSetting'
export const option = {
  //这是组件属性的传值通道
  sidesList: 3,
  fillColor: ["#163D50","#163D50"],
  fillColorNumber: 70,
  colorIsLine: true
}

export default class Config extends PublicConfigClass implements CreateComponentType {
  public key = Polygon.key
   public attr = { ...chartInitConfig, w: 200, h: 200, zIndex: -1 } // 按钮的高度-后来加的  不要可以去掉
  public chartConfig = cloneDeep(Polygon)
  public option = cloneDeep(option)
}
