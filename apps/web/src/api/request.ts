import axios, { type AxiosRequestConfig } from 'axios';
import { ElMessage } from 'element-plus';

interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
}

export interface AppRequestConfig extends AxiosRequestConfig {
  silentError?: boolean;
}

const request = axios.create({
  // 默认走同源 /api，由 Vite/Nginx 代理到后端，局域网访问时只需要开放前端端口。
  baseURL: import.meta.env.VITE_API_BASE_URL ?? '',
  timeout: 10000,
});

request.interceptors.request.use((config) => {
  // 所有业务请求自动携带登录 token，后端 JwtAuthGuard 会统一校验。
  const token = localStorage.getItem('accessToken');
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

request.interceptors.response.use(
  (response) => response,
  (error) => {
    const message = error.response?.data?.message ?? '网络请求失败，请稍后重试';
    if (!error.config?.silentError && error.code !== 'ERR_CANCELED') ElMessage.error(message);
    if (error.response?.status === 401) {
      localStorage.removeItem('accessToken');
      localStorage.removeItem('userInfo');
      if (location.pathname !== '/login') location.href = '/login';
    }
    return Promise.reject(error);
  },
);

export async function apiRequest<T>(config: AppRequestConfig) {
  const response = await request.request<ApiResponse<T>>(config);
  return response.data.data;
}

export default request;
