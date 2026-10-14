import { ChartFrameEnum, PackagesCategoryEnum } from '@/packages/index.d'
import { ImageConfig } from '@/packages/components/Informations/Mores/Image/index'
import { ChatCategoryEnum, ChatCategoryEnumName } from '../index.d'
import { onMounted, ref } from 'vue';
import { baseurlAddress } from "@/utils/request";

let list = sessionStorage.getItem("tupianchunchu") ? JSON.parse(sessionStorage.getItem("tupianchunchu") as any) : [];
let photoConfigList = ref([]);

photoConfigList.value = list.map((i: any) => {
  return {
    ...ImageConfig,
    category: ChatCategoryEnum.PRIVATE,
    categoryName: ChatCategoryEnumName.PRIVATE,
    package: PackagesCategoryEnum.PHOTOS,
    chartFrame: ChartFrameEnum.STATIC,
    redirectComponent: `${ImageConfig.package}/${ImageConfig.category}/${ImageConfig.key}`, // 跳转组件路径规则：packageName/categoryName/componentKey
  
    list: i,
    image: baseurlAddress.urlfile + i.url,
    dataset: baseurlAddress.urlfile + i.url,
    title: i.original_name,
  }
})

export default photoConfigList.value.reverse();
