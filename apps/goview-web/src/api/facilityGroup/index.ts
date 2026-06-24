import { request } from "@/utils/request";

/**
 * 分组关联设备列表
 */
export const model_device_list = async (data: any) => {
    return await request({ url: "/devicegroups/group_device_list", method: "POST", data })
}

/**
 * 查询父分组详情
 */
export const group_info = async (params: any) => {
    return await request({ url:"/devicegroups/group_info", method: "GET", params })
}

/**
 * 保存设备分组排序
 */
export const save_group_order = async (data: any) => {
    return await request({ url: "/devicegroups/save_group_order", method: "POST", data })
}
/**
 * 分组关联设备
 */
export const add_model_device = async (data: any) => {
    return await request({ url: "/devicegroups/add_model_device", method: "POST", data })
}

/**
 * 单个解除分组与设备关联
 */
export const unlink_device = async (data: any) => {
    return await request({ url: "/devicegroups/unlink_device", method: "POST", data })
}
/**
 * 批量解除分组与设备关联
 */
export const batch_unlink_devices = async (data: any) => {
    return await request({ url: "/devicegroups/batch_unlink_devices", method: "POST", data })
}