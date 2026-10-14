import { PublicConfigClass } from '@/packages/public'
import { CreateComponentType } from '@/packages/index.d'
import { CirclePointConfig } from './index'
import cloneDeep from 'lodash/cloneDeep'
import { chartInitConfig } from '@/settings/designSetting'

export interface StateItem {
  value: number
  matchValues: string[]
  outCircleColor: string
  inCircleColor: string
  flash: boolean
  frequency: number
}

export const option = {
  outCircle: 50,
  inCircle: 30,
  outCircleWidth: 10,
  previewState: 0,
  dataset: null,
  states: [
    {
      value: 1,
      matchValues: [''],
      outCircleColor: '#3f5261',
      inCircleColor: '#ffffff',
      flash: false,
      frequency: 100
    },
    {
      value: 2,
      matchValues: [''],
      outCircleColor: '#0a7ae2',
      inCircleColor: '#119bfa',
      flash: false,
      frequency: 100
    }
  ] as StateItem[]
}

export default class Config extends PublicConfigClass implements CreateComponentType {
  public key = CirclePointConfig.key
  public attr = { ...chartInitConfig, w: 30, h: 30, zIndex: 1 }
  public chartConfig = cloneDeep(CirclePointConfig)
  public option = cloneDeep(option)
}
