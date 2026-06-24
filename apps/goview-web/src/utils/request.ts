import router from "@/router";
import axios from "axios";
import { ElMessage } from "element-plus";

// ========== 从环境变量读取配置 ==========
const isProduction = import.meta.env.VITE_IS_PRODUCTION === 'true';

// API 基础地址
export let baseURL = import.meta.env.VITE_API_BASE_URL || '/';

// 组态访问地址（动态获取当前页面的协议和主机名，避免硬编码 localhost）
export let luyou = isProduction
    ? `https://${window.location.host}:8080/`
    : (import.meta.env.VITE_LUYOU_URL || 'http://localhost:3020');
// const protocol = window.location.protocol;
// const hostname = window.location.hostname;
// export let luyou = import.meta.env.VITE_LUYOU_URL || `${protocol}//${hostname}:3020`;

// WebSocket 地址
export let socketUrl = isProduction
    ? `wss://${window.location.host}`
    : (import.meta.env.VITE_SOCKET_URL || 'ws://192.168.1.101');

const instance = axios.create({
    baseURL: baseURL,
    timeout: 50000,
});

// ========== 以下拦截器逻辑保持不变 ==========

let isHandlingTokenExpire = false;
let pendingRequests: { url: string; cancel: (reason?: string) => void }[] = [];
const CancelToken = axios.CancelToken;

const handleTokenExpire = () => {
    if (isHandlingTokenExpire) return;
    isHandlingTokenExpire = true;
    console.warn("token expire handler redirecting to login", {
        href: window.location.href,
        hasToken: !!sessionStorage.getItem("wk_Token")
    });
    ElMessage.error("请重新登录！！！！！");
    pendingRequests.forEach((item) => {
        item.cancel("token过期，取消多余请求");
    });
    pendingRequests = [];
    router.push("/login").finally(() => {
        setTimeout(() => {
            isHandlingTokenExpire = false;
        }, 1000);
    });
};

instance.interceptors.request.use(
    async function (config) {
        if (!config.headers["Content-Type"]) {
            if (config.method === "post") {
                config.headers["content-Type"] = "application/json;charset=utf-8";
            } else if (config.method === "get") {
                config.headers["content-Type"] = "application/json;charset=utf-8";
            }
        }

        // 开发环境去掉 /api 前缀，生产环境保留
        if (!isProduction) {
            config.url = config.url?.replace("/api", "");
        }

        let { token } = sessionStorage.getItem("wk_Token")
            ? JSON.parse(sessionStorage.getItem("wk_Token") as any)
            : { token: void 0 };
        window.postMessage(token, "*");
        localStorage.setItem("message", JSON.stringify({ text: token, time: Date.now() }));
        if (token) {
            config.headers.Authorization = `Bearer ${token}`;
        }

        config.cancelToken = new CancelToken((cancel) => {
            pendingRequests.push({
                url: config.url as string,
                cancel: cancel,
            });
        });

        return config;
    },
    function (error) {
        return Promise.reject(error);
    }
);

instance.interceptors.response.use(
    function (response) {
        if (response.data instanceof Blob) {
            return response;
        }
        if (response.data.code == 400) {
            ElMessage.error(response.data.msg);
        } else if (response.data.code == 999999999) {
            handleTokenExpire();
            return Promise.reject("token过期，已跳转登录页");
        } else if (response.data.code == 402) {
            ElMessage.error(response.data.msg);
        } else if (response.data.code == 403) {
            ElMessage.error(response.data.msg);
        } else {
            pendingRequests = pendingRequests.filter((item) => item.url !== response.config.url);
            return response.data;
        }
    },
    function (error) {
        if (error.code == "ECONNABORTED") {
            ElMessage.error("请求超时！");
            throw "请求超时！" + error;
        } else if (error.code == "ERR_NETWORK") {
            ElMessage.error("网络错误,请联系管理员！");
            throw "网络错误，请联系管理员！！";
        } else if (error.response?.status == 401) {
            handleTokenExpire();
            throw "token过期，无访问权限！";
        } else if (error.response?.status == 500) {
            ElMessage.error("后端服务器500错误！");
            throw "后端服务器500错误！";
        }

        if (!axios.isCancel(error)) {
            pendingRequests = pendingRequests.filter((item) => item.url !== error.config?.url);
        }

        return Promise.reject(error);
    }
);

function request({ url, method, data, params, headers, responseType }: { url: string; method: string; data?: any; params?: any; headers?: any; responseType?: any }) {
    return new Promise((resolve, reject) => {
        instance({ url: url, method: method, data: data, params: params, headers: headers, responseType: responseType })
            .then((response) => {
                resolve(response);
            })
            .catch((error) => {
                reject(error);
            });
    });
}

export { request };
