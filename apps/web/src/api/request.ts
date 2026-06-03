import axios, { type AxiosRequestConfig } from 'axios';
import { ElMessage } from 'element-plus';

interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
}

const request = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL ?? `${window.location.protocol}//${window.location.hostname}:3100`,
  timeout: 10000,
});

request.interceptors.request.use((config) => {
  const token = localStorage.getItem('accessToken');
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

request.interceptors.response.use(
  (response) => response,
  (error) => {
    const message = error.response?.data?.message ?? '网络请求失败，请稍后重试';
    ElMessage.error(message);
    if (error.response?.status === 401) {
      localStorage.removeItem('accessToken');
      localStorage.removeItem('userInfo');
      if (location.pathname !== '/login') location.href = '/login';
    }
    return Promise.reject(error);
  },
);

export async function apiRequest<T>(config: AxiosRequestConfig) {
  const response = await request.request<ApiResponse<T>>(config);
  return response.data.data;
}

export default request;
