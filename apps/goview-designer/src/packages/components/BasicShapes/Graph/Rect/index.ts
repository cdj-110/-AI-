import { ConfigType, PackagesCategoryEnum, ChartFrameEnum } from '@/packages/index.d'
import { ChatCategoryEnum, ChatCategoryEnumName } from '../../index.d'

export const Rect: ConfigType = {
  key: 'Rect',
  chartKey: 'VRect',
  conKey: 'VCRect',
  title: '矩形-01',
  category: ChatCategoryEnum.Graph,
  categoryName: ChatCategoryEnumName.Graph,
  package: PackagesCategoryEnum.BasicShapes,
  chartFrame: ChartFrameEnum.STATIC,
  image: 'border.png'
}
