import { ConfigType, PackagesCategoryEnum, ChartFrameEnum } from '@/packages/index.d'
import { ChatCategoryEnum, ChatCategoryEnumName } from '../../index.d'

export const InputFun: ConfigType = {
  key: 'InputFun',
  chartKey: 'VInputFun',
  conKey: 'VCInputFun',
  title: '输入框-01',
  category: ChatCategoryEnum.Graph,
  categoryName: ChatCategoryEnumName.Graph,
  package: PackagesCategoryEnum.BasicShapes,
  chartFrame: ChartFrameEnum.STATIC,
  image: 'shurukuang.png'
}
