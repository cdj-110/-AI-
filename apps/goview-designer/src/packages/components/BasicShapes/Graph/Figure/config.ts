import {  PublicConfigClass } from '@/packages/public'
import { CreateComponentType } from '@/packages/index.d'
import { Figure } from './index'
import cloneDeep from 'lodash/cloneDeep'
import { chartInitConfig } from '@/settings/designSetting'

export const option = {
  //这是组件属性的传值通道
  borderColor: "#3774B6ff",
  chuxi: 5,
  lineStyle: 'solid',
  lineCap: 'butt',
  startArrow: false,
  endArrow: false
}

export default class Config extends PublicConfigClass implements CreateComponentType {
  public key = Figure.key
  public attr = { ...chartInitConfig, w: 300, h: 5, zIndex: -100 }
  public chartConfig = cloneDeep(Figure)
  public option = cloneDeep(option)
}
