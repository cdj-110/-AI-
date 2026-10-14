import cloneDeep from 'lodash/cloneDeep'
import { PublicConfigClass } from '@/packages/public'
import { CreateComponentType } from '@/packages/index.d'
import { chartInitConfig } from '@/settings/designSetting'
import { TablesAdvancedConfig } from './index'
import dataJson from '../TablesBasic/data.json'

const { dimensions, source } = dataJson
export const option = {
  dataset: { dimensions, source },
  pagination: { page: 1, pageSize: 10, itemCount: 0 },
  query: {
    keywordPlaceholder: '搜索设备名称或点位值',
    showTimeRange: true
  },
  align: 'center',
  style: {
    border: 'on',
    singleColumn: 'off',
    singleLine: 'off',
    bottomBordered: 'on',
    striped: 'on',
    fontSize: 14,
    borderWidth: 0,
    borderColor: 'black',
    borderStyle: 'solid'
  }
}

export default class Config extends PublicConfigClass implements CreateComponentType {
  public key = TablesAdvancedConfig.key
  public attr = { ...chartInitConfig, w: 900, h: 420 }
  public chartConfig = cloneDeep(TablesAdvancedConfig)
  public option = cloneDeep(option)
}
