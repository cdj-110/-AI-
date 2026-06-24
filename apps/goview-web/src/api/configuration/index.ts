

import { request } from "@/utils/request";

/**
 * 创建数据大屏(名称)
 */
export const apiV1ConfigCatalog = async (data: any) => {
    return await request({ url: "/conf/config/catalog", method: "POST", data })
}

/**
 * 修改数据大屏(名称)
 */
export const apiconfConfigCatalogRename = async (data: any) => {
    return await request({ url: "/conf/config/catalog/rename", method: "POST", data })
}

/**
 * 查询数据大屏
 */
export const getApiV1ConfigCatalog = async (params: any) => {
    return await request({ url: "/conf/config/catalog", method: "GET", params })
}

/**
 * 删除数据大屏
 */
export const apiV1ConfigDelete = async (data: any) => {
    return await request({ url: "/conf/config/delete_entry", method: "POST", data })
}

/**
 * 将数据大屏添加到工作台
 */
export const configCatalogWorkbenchPin = async (data: any) => {
    return await request({ url: "/conf/config/catalog/workbench/pin", method: "POST", data })
}

/**
 * 将数据大屏从工作台移除
 */
export const configCatalogWorkbenchUnpin = async (data: any) => {
    return await request({ url: "/conf/config/catalog/workbench/unpin", method: "POST", data })
}
/**
 * 复制组态信息
 */
export const copy = async (data: any) => {
    return await request({ url: "/conf/config/catalog/copy", method: "POST", data })
}
/**
 * 复制组态信息
 */
export const search = async (params: any) => {
    return await request({ url: "/conf/config/catalog/search", method: "GET", params })
}
/**
 * 生成分享链接
 */
export const create = async (data: any) => {
    return await request({ url: "/conf/share/create", method: "POST", data })
}
// /**
//  * 生成分享链接
//  */
// export const bootstrap = async (params: any) => {
//     return await request({ url: "/conf/share/bootstrap", method: "FET", params })
// }
/**
 * 生成分享链接
 */
export const bootstrap = async (params: any) => {
  // 1. 单独获取token（根据你项目实际存储位置调整）
  const token = localStorage.getItem('token') || sessionStorage.getItem('token');

  return await request({ 
    url: "/conf/share/bootstrap", 
    method: "GET", // 顺便修正你这里的笔误：FET → GET，不然接口会405错误
    params,
    // 2. 关键：只给这个接口单独添加请求头
    headers: {
      // 标准JWT token格式（90%项目用这个）
      Authorization: `Bearer ${token}`
      
      // 如果后端要求用X-Token字段，就注释上面，用下面这行
      // 'X-Token': token
    }
  })
}
/**
 * 撤销分享链接
 */
export const revoke = async (data: any) => {
    return await request({ url: "/conf/share/revoke", method: "POST", data })
}

