import { ChartFrameEnum, ConfigType, PackagesCategoryEnum } from '@/packages/index.d'
import { ImageConfig } from '@/packages/components/Informations/Mores/Image/index'
import { ChatCategoryEnum, ChatCategoryEnumName } from '../index.d'
import { setLocalStorage, getLocalStorage, goDialog } from '@/utils'
import { StorageEnum } from '@/enums/storageEnum'
import { FileTypeEnum } from '@/enums/fileTypeEnum'
import { backgroundImageSize } from '@/settings/designSetting'
import { usePackagesStore } from '@/store/modules/packagesStore/packagesStore'
import { apiV1MediaImages, apiV1MediaImagesList } from '@/api/index'
import { ref } from 'vue'
import { baseurlAddress } from "@/utils/request";

const StoreKey = StorageEnum.GO_USER_MEDIA_PHOTOS

const userPhotosList: ConfigType[] = getLocalStorage(StoreKey) || []

let userId = ref(sessionStorage.getItem('wk_UserId'));

const addConfig = {
    ...ImageConfig,
    category: ChatCategoryEnum.PRIVATE,
    categoryName: ChatCategoryEnumName.PRIVATE,
    package: PackagesCategoryEnum.PHOTOS,
    chartFrame: ChartFrameEnum.STATIC,
    title: '点击上传图片',
    image: 'upload.png',
    redirectComponent: `${ImageConfig.package}/${ImageConfig.category}/${ImageConfig.key}`, // 跳转组件路径规则：packageName/categoryName/componentKey
    disabled: true,
    configEvents: {
        // 点击上传事件
        addHandle: (photoConfig: ConfigType) => {
            goDialog({
                message: `图片需小于 ${backgroundImageSize}M。当前图片暂存上限5M。`,
                transformOrigin: 'center',
                onPositiveCallback: () => {
                    const input = document.createElement('input')
                    input.type = 'file'
                    input.accept = '.png,.jpg,.jpeg,.gif' // 这里只允许部分图片类型
                    input.onchange = async () => {
                        if (!input.files || !input.files.length) return
                        const file = input.files[0]
                        const { name, size, type } = file
                        if (size > 1024 * 1024 * backgroundImageSize) {
                            window['$message'].warning(`图片超出 ${backgroundImageSize}M 限制，请重新上传！`)
                            return false
                        }
                        if (type !== FileTypeEnum.PNG && type !== FileTypeEnum.JPEG && type !== FileTypeEnum.GIF) {
                            window['$message'].warning('文件格式不符合，请重新上传！')
                            return false
                        }
                        let fileList = new FormData();
                        fileList.append("file", file);
                        apiV1MediaImages(fileList, { user_id: userId.value }).then(async (res: any) => {
                            window['$message'].success('文件上传成功！');
                              let imgRes:any = await apiV1MediaImagesList({ user_id: userId.value });
                            sessionStorage.setItem("tupianchunchu", JSON.stringify(imgRes.items));
                            const newPhoto = {
                                ...ImageConfig,
                                category: ChatCategoryEnum.PRIVATE,
                                categoryName: ChatCategoryEnumName.PRIVATE,
                                package: PackagesCategoryEnum.PHOTOS,
                                chartFrame: ChartFrameEnum.STATIC,
                                redirectComponent: `${ImageConfig.package}/${ImageConfig.category}/${ImageConfig.key}`, // 跳转组件路径规则：packageName/categoryName/componentKey

                                list: imgRes.items.filter((item: any) => item.url == res.url)[0],
                                title: imgRes.items.filter((item: any) => item.url == res.url)[0].original_name,
                                image: baseurlAddress.urlfile + res.url,
                                dataset: baseurlAddress.urlfile + res.url,
                            }
                            const packagesStore = usePackagesStore()
                            packagesStore.addPhotos(newPhoto, 1)
                            userPhotosList.unshift(newPhoto)
                        })
                    }
                    input.click();
                }
            })
        }
    }
}

export default [addConfig, ...userPhotosList]
