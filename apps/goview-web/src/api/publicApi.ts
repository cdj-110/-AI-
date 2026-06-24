import { request } from "@/utils/request";

/**
 * 资产管理 - 入库 - 入库登记备件下拉框
 */
export const getSparePartTypeList = async (data:any) => {
    return await request({ url: `${import.meta.env.VITE_NO_ZUTAI_API}/auth/login`, method: "POST", data, headers: { 'Content-Type': 'application/x-www-form-urlencoded' } })
}