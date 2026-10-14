import { request } from "@/utils/request";

/**
 * 查询设备列表
 */
export const deviceinfo = async (data: any) => {
    return await request({ url: "/deviceinfo/deviceinfo", method: "POST", data })
}

/**
 * 属性
 */
export const device_detail_function = async (data: any) => {
    return await request({ url: "/deviceinfo/device_detail_function", method: "POST", data })
}

/**
 * 存储点位信息
 */
export const configurationPointBindings = async (data: any) => {
    return await request({ url: "/configuration/point-bindings", method: "POST", data })
}

/**
 * 存储组态数据
 */
export const apiV1Config = async (data: any) => {
    return await request({ url: "/conf/config", method: "POST", data })
}

/**
 * 上传图片接口
 */
export const apiV1MediaImages = async (data:any,params:any,headers: Record<string, string> = { 'content-Type': 'multipart/form-data' }) => {
    return await request({ url: "/conf/media/images", method: "POST", data, params, headers })
}

/**
 * 查询图片接口
 */
export const apiV1MediaImagesList = async (params:any) => {
    return await request({ url: "/conf/media/images/list", method: "GET", params})
}

/**
 * 删除图片接口
 */
export const apiV1MediaImagesDelete = async (data:any) => {
    return await request({ url: "/conf/media/images/delete", method: "POST", data })
}

/**
 * 渲染组态界面
 */
export const apiV1configDatas_test = async (params:any) => {
    return await request({ url: "/conf/config/datas_unlocked", method: "GET", params })
}

/**
 * 渲染组态界面(带锁的)
 */
export const apiV1ConfigDatas = async (params:any) => {
    return await request({ url: "/conf/config/datas", method: "GET", params })
}