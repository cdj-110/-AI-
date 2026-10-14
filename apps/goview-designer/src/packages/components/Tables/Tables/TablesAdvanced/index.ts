import { ConfigType, PackagesCategoryEnum, ChartFrameEnum } from '@/packages/index.d'
import { ChatCategoryEnum, ChatCategoryEnumName } from '../../index.d'

export const TablesAdvancedConfig: ConfigType = {
  key: 'TablesAdvanced',
  chartKey: 'VTablesAdvanced',
  conKey: 'VCTablesAdvanced',
  title: '高级分页表格',
  category: ChatCategoryEnum.TABLE,
  categoryName: ChatCategoryEnumName.TABLE,
  package: PackagesCategoryEnum.TABLES,
  chartFrame: ChartFrameEnum.COMMON,
  image: 'tables_basic.png'
}
