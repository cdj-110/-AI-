import { ConfigType, PackagesCategoryEnum, ChartFrameEnum } from '@/packages/index.d'
import { ChatCategoryEnum, ChatCategoryEnumName } from '../../index.d'

export const SwitchFun: ConfigType = {
  key: 'SwitchFun',
  chartKey: 'VSwitchFun',
  conKey: 'VCSwitchFun',
  title: '开关-01',
  category: ChatCategoryEnum.Graph,
  categoryName: ChatCategoryEnumName.Graph,
  package: PackagesCategoryEnum.BasicShapes,
  chartFrame: ChartFrameEnum.COMMON,
  image: 'kaiguan.png'
}
