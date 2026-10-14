import {  PublicConfigClass } from '@/packages/public'
import { CreateComponentType } from '@/packages/index.d'
import { Rect } from './index'
import cloneDeep from 'lodash/cloneDeep'
import { chartInitConfig } from '@/settings/designSetting'

export const option = {
  borderColor: "#3774B6ff",
  borderNumber: 3,
  borderType: "solid",
  backgroundLinearColor: ["#163D50","#163D50"],
  backgroundLinearNumber: 70,
  radius: [0, 0, 0, 0],
  colorIsLine: 1,
  showBorder: false,
}

export default class Config extends PublicConfigClass implements CreateComponentType {
  public key = Rect.key
  public attr = { ...chartInitConfig, w: 300, h: 200, zIndex: -1 } // 按钮的高度-后来加的  不要可以去掉
  public chartConfig = cloneDeep(Rect)
  public option = cloneDeep(option)
}
