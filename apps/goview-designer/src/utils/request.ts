import axios from "axios";
import { readPlatformToken } from '@/utils/platformAuth';
import { recoverPlatformSession, sessionRecovery } from '@/utils/sessionRecovery';
import { requestErrorMessage } from '@/utils/requestErrorMessage';

const isProduction = import.meta.env.PROD;
const DEV_BASE_IP = import.meta.env.VITE_DEV_BASE_IP || "192.168.1.101";
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || "";

const luyouUrl = isProduction
  ? `https://${window.location.host}:8080/`
  : import.meta.env.VITE_LUYOU_URL || `https://${DEV_BASE_IP}:8080/`;

const urlfileUrl = isProduction
  ? `https://${window.location.host}:8080`
  : import.meta.env.VITE_URLFILE || `https://${DEV_BASE_IP}:8080`;

const wssUrl = isProduction
  ? `wss://${window.location.host}`
  : import.meta.env.VITE_WSS_URL_TEST || `wss://${DEV_BASE_IP}`;

const ws8000Url = isProduction
  ? `wss://${window.location.host}`
  : import.meta.env.VITE_WS8000_URL || `ws://${DEV_BASE_IP}:8000`;

export let baseurlAddress = {
  url: API_BASE_URL,
  urlfile: urlfileUrl,
  luyou: luyouUrl,
  wss: wssUrl.split(":")[0] + ":" + wssUrl.split(":")[1],
  ws8000: ws8000Url
};

const instance = axios.create({
  baseURL: baseurlAddress.url,
  timeout: 50000
});

let pendingRequests = [];
const CancelToken = axios.CancelToken;

const retryAfterLogin = async (config: any) => {
  if (config.__wkRetried) throw new Error('登录已恢复，但当前请求仍无法通过认证');
  const token = await recoverPlatformSession();
  if (!['get', 'head'].includes(String(config.method || 'get').toLowerCase())) {
    const message = '登录已恢复，请重新点击保存或执行刚才的操作';
    window['$message']?.warning(message);
    throw new Error(message);
  }
  config.__wkRetried = true;
  config.headers.Authorization = `Bearer ${token}`;
  return instance.request(config);
};

const readStoredToken = () => {
  return readPlatformToken();
};

instance.interceptors.request.use(
  async function (config) {
    if (sessionRecovery.paused) {
      // 初始化项目配置只执行一次，等待登录后继续；定时数据请求继续暂停。
      if (config.method === 'get' && ['/conf/config/datas_unlocked', '/conf/config/datas'].includes(config.url || '')) {
        await recoverPlatformSession();
      } else {
        throw new axios.Cancel('登录已过期，数据请求已暂停');
      }
    }
    const platformApiBase = sessionStorage.getItem('wk_ApiBaseUrl')
    if (platformApiBase) config.baseURL = platformApiBase
    if (!config.headers["Content-Type"]) {
      if (config.method === "post" || config.method === "get") {
        config.headers["content-Type"] = "application/json;charset=utf-8";
      }
    }

    let token = "";
    const route = window.route;
    if (route?.params?.token) {
      token = route.params.token;
      sessionStorage.setItem("wk_UserId", route.params.userId);
      sessionStorage.setItem("wk_Config_id", route.params.config_id);
    } else {
      token = readStoredToken();
    }

    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }

    config.cancelToken = new CancelToken(cancel => {
      pendingRequests.push({
        url: config.url,
        cancel
      });
    });

    return config;
  },
  function (error) {
    return Promise.reject(error);
  }
);

instance.interceptors.response.use(
  async function (response) {
    if (response.data.code == 400) {
      window["$message"].warning(response.data.msg);
    } else if (response.data.code == 999999999) {
      return retryAfterLogin(response.config);
    } else if (response.data.code == 402) {
      window["$message"].warning(response.data.msg);
    } else if (response.data.code == 403) {
      window["$message"].warning(response.data.msg);
    } else {
      pendingRequests = pendingRequests.filter(item => item.url !== response.config.url);
      return response.data;
    }
  },
  async function (error) {
    if (error.code == "ECONNABORTED") {
      window["$message"].warning("request timeout");
      throw "request timeout: " + error;
    } else if (error.code == "ERR_NETWORK") {
      window["$message"].warning("network error");
      throw "network error";
    } else if (error.response?.status == 401) {
      pendingRequests = pendingRequests.filter(item => item.url !== error.config?.url);
      return retryAfterLogin(error.config);
    } else if (error.response?.status == 400 || error.response?.status == 413 || error.response?.status >= 500) {
      const message = requestErrorMessage(error);
      pendingRequests = pendingRequests.filter(item => item.url !== error.config?.url);
      window["$message"].warning(message);
      throw Object.assign(new Error(message), { status: error.response.status });
    }

    if (!axios.isCancel(error)) {
      pendingRequests = pendingRequests.filter(item => item.url !== error.config?.url);
    }

    return Promise.reject(error);
  }
);

function request({
  url,
  method,
  data,
  params,
  headers,
  responseType
}) {
  return new Promise((resolve, reject) => {
    instance({
      url,
      method,
      data,
      params,
      headers,
      responseType,
      transformRequest:
        data instanceof FormData
          ? undefined
          : [
              data => {
                const replacer = (_key, value) =>
                  value === undefined ? null : value;
                return JSON.stringify(data, replacer);
              }
            ]
    })
      .then(response => {
        resolve(response);
      })
      .catch(error => {
        reject(error);
      });
  });
}

export { request };
