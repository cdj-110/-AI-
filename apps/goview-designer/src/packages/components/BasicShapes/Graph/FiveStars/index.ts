import { ConfigType, PackagesCategoryEnum, ChartFrameEnum } from '@/packages/index.d'
import { ChatCategoryEnum, ChatCategoryEnumName } from '../../index.d'

export const FiveStars: ConfigType = {
  key: 'FiveStars',
  chartKey: 'VFiveStars',
  conKey: 'VCFiveStars',
  title: '五角星-01',
  category: ChatCategoryEnum.Graph,
  categoryName: ChatCategoryEnumName.Graph,
  package: PackagesCategoryEnum.BasicShapes,
  chartFrame: ChartFrameEnum.STATIC,
  image: 'wujiaoxing.png'
}
