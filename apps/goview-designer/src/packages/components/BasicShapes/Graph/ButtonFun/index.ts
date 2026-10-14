import { ConfigType, PackagesCategoryEnum, ChartFrameEnum } from '@/packages/index.d'
import { ChatCategoryEnum, ChatCategoryEnumName } from '../../index.d'

export const ButtonFun: ConfigType = {
  key: 'ButtonFun',
  chartKey: 'VButtonFun',
  conKey: 'VCButtonFun',
  title: '按钮-01',
  category: ChatCategoryEnum.Graph,
  categoryName: ChatCategoryEnumName.Graph,
  package: PackagesCategoryEnum.BasicShapes,
  chartFrame: ChartFrameEnum.STATIC,
  image: 'annv.png'
}
