import { PublicConfigClass } from '@/packages/public'
import { CreateComponentType } from '@/packages/index.d'
import { NumberConfig } from './index'
import cloneDeep from 'lodash/cloneDeep'
import { chartInitConfig } from '@/settings/designSetting'

export const option = {
  // 数据说明
  dataset: 10000,
  from: 0,
  dur: 3,
  precision: 0,
  showSeparator: true,
  numberSize: 34,
  numberColor: '#4a9ef8',
  prefixText: '￥',
  prefixColor: '#4a9ef8',
  suffixText: '元',
  suffixColor: '#4a9ef8',
  donghuazhaungtai: "打开动画"
}

export default class Config extends PublicConfigClass implements CreateComponentType {
  public key = NumberConfig.key
  public attr = { ...chartInitConfig, w: 240, h: 45, zIndex: -1 } // 按钮的高度-后来加的  不要可以去掉
  public chartConfig = cloneDeep(NumberConfig)
  public option = cloneDeep(option)
}
