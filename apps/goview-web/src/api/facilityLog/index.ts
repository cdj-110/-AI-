import { request } from "@/utils/request";

/**
 * 添加设备
 */
export const historical_devicelog = async (data: any) => {
    return await request({ url: "/mqtt_collection/log/historical_devicelog", method: "POST", data })
}
/**
 * 设备列表-详情-获取所有日志类型
 */
export const all_types = async () => {
    return await request({ url: "/mqtt_collection/all_types", method: "GET" })
}