import { request } from "@/utils/request";

/**
 * 工作台设备概览统计
 */
export const workbench_overview = async (data: any) => {
    return await request({ url: "/deviceinfo/workbench_overview", method: "POST", data })
}