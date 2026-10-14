import { ConfigType, PackagesCategoryEnum, ChartFrameEnum } from '@/packages/index.d'
import { ChatCategoryEnum, ChatCategoryEnumName } from '../../index.d'

export const TextFun: ConfigType = {
  key: 'TextFun',
  chartKey: 'VTextFun',
  conKey: 'VCTextFun',
  title: '文本-01',
  category: ChatCategoryEnum.Graph,
  categoryName: ChatCategoryEnumName.Graph,
  package: PackagesCategoryEnum.BasicShapes,
  chartFrame: ChartFrameEnum.STATIC,
  image: 'text.png'
}
