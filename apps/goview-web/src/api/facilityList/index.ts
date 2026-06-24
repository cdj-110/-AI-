import { request } from "@/utils/request";

/**
 * 添加设备
 */
export const addDevice = async (data: any) => {
    return await request({ url: "/deviceinfo/add_device", method: "POST", data })
}

/**
 * 查询分组的父分组(传入group_id则是筛选对应的子分组)
 */
export const groupList = async (data: any) => {
    return await request({ url: "/devicegroups/group_list", method: "POST", data })
}

/**
 * 新增分组
 */
export const addDevicegroup = async (data: any) => {
    return await request({ url: "/devicegroups/add_devicegroup", method: "POST", data })
}

/**
 * 修改分组详情信息内容
 */
export const update_devicegroup = async (data: any) => {
    return await request({ url: "/devicegroups/update_devicegroup", method: "POST", data })
}

/**
 * 创建设备模型
 */
export const addDevicemodel = async (data: any) => {
    return await request({ url: "/devicemodel/add_devicemodel", method: "POST", data })
}

/**
 * 查询协议/设备类型/接入方式
 */
export const modelDatas = async () => {
    return await request({ url: "/devicemodel/model_datas", method: "GET" })
}

/**
 * 查询设备列表
 */
export const deviceinfo = async (data: any) => {
    return await request({ url: "/deviceinfo/deviceinfo", method: "POST", data })
}

/**
 * 获取所有子设备列表
 */
export const sub_device_list = async (data: any) => {
    return await request({ url: "/deviceinfo/sub_device_list", method: "POST", data })
}

/**
 * 查询设备详情头部
 */
export const device_detail = async (params: any) => {
    return await request({ url: "/deviceinfo/device_detail", method: "GET", params })
}

/**
 * 修改设备详情头部
 */
export const update_device_detail = async (data: any) => {
    return await request({ url: "/deviceinfo/update_device_detail", method: "PUT", data })
}

/**
 * 查询设备地域信息，查询省市区
 */
export const locationProvinces = async (data: any) => {
    return await request({ url: "/deviceinfo/location_provinces", method: "POST", data })
}

/**
 * 根据设备地域信息查询设备详情
 */
export const locationDevices = async (data: any) => {
    return await request({ url: "/deviceinfo/location_devices", method: "POST", data })
}

/**
 * 删除分组
 */
export const delete_devicegroup = async (data: any) => {
    return await request({ url: "/devicegroups/delete_devicegroup", method: "POST", data })
}

/**
 * 子设备列表
 */
export const device_detail_sublist = async (params: any) => {
    return await request({ url: "/deviceinfo/device_detail_sublist", method: "GET",params })
}

/**
 * 删除分组
 */
export const device_detail_add_subdevice = async (data: any) => {
    return await request({ url: "/deviceinfo/device_detail_add_subdevice", method: "POST", data })
}

/**
 * 删除分组
 */
export const delete_device = async (data: any) => {
    return await request({ url: "/deviceinfo/hard_delete_device", method: "POST", data })
}
/**
 * 删除设备（软删除）
 */
export const soft_delete_device = async (data: any) => {
    return await request({ url: "/deviceinfo/soft_delete_device", method: "POST", data })
}

/**
 * 查询设备列表 - 功能属性列表
 */
export const device_detail_function = async (data: any) => {
    return await request({ url: "/deviceinfo/device_detail_function", method: "POST", data })
}
/**
 * 设备列表-详情-设备连网
 */
export const device_detail_connect = async (params: any) => {
    return await request({ url: "/deviceinfo/device_detail_connect", method: "GET",params })
}

/**
 * 设备列表 - 恢复设备
 */
export const recovery_device = async (data: any) => {
    return await request({ url: "/deviceinfo/recovery_device", method: "POST", data })
}
/**
 * 设备列表 - 详情 - 功能属性 - 属性历史数据
 */
export const historical_deviceanalysis = async (data: any) => {
    return await request({ url: "/mqtt_collection/data/historical_deviceanalysis", method: "POST", data })
}
/**
 * 设备列表 - 详情 - 功能属性 - 属性历史数据  - 属性日志导出
 */
export const exports = async (data: any) => {
    return await request({ url: "/mqtt_collection/data/historical_deviceanalysis/export", method: "POST", data,responseType: 'blob' })
}
/**
 * 设备列表 - 详情 - 功能属性 - 属性历史数据  - 属性日志导出
 */
export const function_event_log = async (data: any) => {
    return await request({ url: "/mqtt_collection/event/log/function_event_log", method: "POST",data })
}
/**
 * 设备列表 - 详情 - 功能下发
 */
export const push_attributes = async (data: any) => {
    return await request({ url: "/deviceinfo/push_attributes", method: "POST",data })
}



