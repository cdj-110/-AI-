import { request } from "@/utils/request";
/**
 * 日志列表
 */
export const operation_log_list = async (data: any) => {
    return await request({ url: "/logger/operation_log_list", method: "POST", data })
}