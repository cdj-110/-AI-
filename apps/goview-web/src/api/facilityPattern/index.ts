import { request } from "@/utils/request";
/**
 * 设备模型列表
 */
export const model_list = async (data: any) => {
    return await request({ url: "/devicemodel/model_list", method: "POST", data })
}
/**
 * 获取微控产品模型列表
 */
export const product_models = async () => {
    return await request({ url: "/devicemodel/product_models", method: "GET" })
}
/**
 * 设备模型删除
 */
export const delete_devicemodel = async (data: any) => {
    return await request({ url: "/devicemodel/delete_devicemodel", method: "POST", data })
}
/**
 * 关联设备列表
 */
export const model_device_list = async (data: any) => {
    return await request({ url: "/devicemodel/model_device_list", method: "POST", data })
}

/**
 * 关联设备
 */
export const add_model_device = async (data: any) => {
    return await request({ url: "/devicemodel/add_model_device", method: "POST", data })
}
/**
 * 设备模型详情
 */
export const model_info = async (params: any) => {
    return await request({ url: "/devicemodel/model_info", method: "GET", params })
}

/**
 * 设备模型-功能定义参数列表
 */
export const get_parameter_info = async () => {
    return await request({ url: "/devicemodel/get_parameter_info", method: "GET" })
}

/**
 * 功能属性列表
 */
export const get_property = async (data: any) => {
    return await request({ url: "/devicemodel/get_property", method: "POST", data })
}
/**
 * 添加/编辑功能属性
 */
export const add_property = async (data: any) => {
    return await request({ url: "/devicemodel/add_property", method: "POST", data })
}
/**
 * 修改模型功能定义属性
 */
export const edit_property = async (data: any) => {
    return await request({ url: "/devicemodel/edit_property", method: "PUT", data })
}
/**
 * 添加设备模型功能定义事件
 */
export const add_event = async (data: any) => {
    return await request({ url: "/devicemodel/add_event", method: "POST", data })
}
/**
 * 功能事件列表
 */
export const model_event_list = async (data: any) => {
    return await request({ url: "/devicemodel/model_event_list", method: "POST", data })
}
/**
 * 修改模型功能定义属性
 */
export const edit_model_event = async (data: any) => {
    return await request({ url: "/devicemodel/edit_model_event", method: "PUT", data })
}
/**
 * 修改模型详情信息
 */
export const update_device_model = async (data: any) => {
    return await request({ url: "/devicemodel/update_device_model", method: "POST", data })
}
/**
 * 单个解除模型与设备关联
 */
export const unlink_model_device = async (data: any) => {
    return await request({ url: "/devicemodel/unlink_model_device", method: "POST", data })
}
/**
 * 批量解除模型与设备关联
 */
export const batch_unlink_model_devices = async (data: any) => {
    return await request({ url: "/devicemodel/batch_unlink_model_devices", method: "POST", data })
}
/**
 * 删除模型功能定义属性
 */
export const delete_property = async (data: any) => {
    return await request({ url: "/devicemodel/delete_property", method: "POST", data })
}
/**
 * 删除模型功能定义事件
 */
export const delete_event = async (data: any) => {
    return await request({ url: "/devicemodel/delete_event", method: "POST", data })
}
/**
 * 设备列表详情-绑定模型
 */
export const device_detail_bind_model = async (data: any) => {
    return await request({ url: "/deviceinfo/device_detail_bind_model", method: "POST", data })
}


