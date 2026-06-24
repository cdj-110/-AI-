import { request } from "@/utils/request";
/**
 * 获取webSocket连接token
 */
export const wsTokenApi = async (params:any) => {
    return await request({ url: "/mqtt_collection/status/realtime/all/devices", method: "GET", params })
}
/**
 * 获取菜单接口
 */
export const cloudInfo = async () => {
    return await request({ url: "/cloud/info", method: "get" })
}
/**
 * 密码登录
 */
export const login = async (data: any) => {
    return await request({ url: "/auth/login_pw", method: "POST", data, headers: { 'Content-Type': 'application/x-www-form-urlencoded' } })
}
/**
 * 免密登录
 */
export const loginCode = async (data: any) => {
    return await request({ url: "/auth/login_code", method: "POST", data })
}
/**
 * 发送验证码
 */
export const sendcode = async (data: any) => {
    return await request({ url: "/user/sendcode", method: "POST", data })
}
/**
 * 注册用户
 */
export const register = async (data: any) => {
    return await request({ url: "/user/register", method: "POST", data })
}
/**
 * 忘记密码
 */
export const forgetpwd = async (data: any) => {
    return await request({ url: "/user/forgetpwd", method: "POST", data })
}

/**
 * 获取图片验证码
 */
export const get_verification_code = async () => {
    return await request({ url:"/auth/get_verification_code", method: "GET" })
}

/**
 * 退出登录
 */
export const logout = async (data: any) => {
    return await request({ url: "/auth/logout", method: "POST", data })
}

/**
 * 微信登录二维码
 */
export const loginrul = async () => {
    return await request({ url: "/auth/loginrul", method: "GET" })
}
/**
 * 微信登录
 */
export const login_wechat = async (data: any) => {
    return await request({ url: "/auth/login_wechat", method: "POST", data })
}
/**
 * 微信注册
 */
export const wechat_binding = async (data: any) => {
    return await request({ url: "/auth/wechat_binding", method: "POST", data })
}
/**
 * 获取用户信息
 */
export const user_info = async (code: any) => {
    return await request({ url: `/user/user_info?user_id=${code}`, method: "GET" })
}
/**
 * 修改用户信息
 */
export const update_user_infog = async (data: any) => {
    return await request({ url: "/user/update_user_info", method: "POST", data })
}
/**
 * 设置密码
 */
export const set_pwd = async (data: any) => {
    return await request({ url: "/user/set_pwd", method: "POST", data })
}
/**
 * 修改密码
 */
export const update_pwd = async (data: any) => {
    return await request({ url: "/user/update_pwd", method: "POST", data })
}
/**
 * 告警账号校验
 */
export const check_alert_account = async (data: any) => {
    return await request({ url: "/user/check_alert_account", method: "POST", data })
}
/**
 * 告警账号校验
 */
export const set_alert_account = async (data: any) => {
    return await request({ url: "/user/set_alert_account", method: "POST", data })
}





