import { defineStore } from 'pinia';
import { apiRequest } from '../api/request';

export interface UserInfo {
  id: string;
  username: string;
  nickname?: string;
  role: string;
  tenantId: string | null;
}

interface LoginPayload {
  username: string;
  password: string;
}

interface LoginResponse {
  accessToken: string;
  user: UserInfo;
}

export const useUserStore = defineStore('user', {
  state: () => ({
    token: localStorage.getItem('accessToken') ?? '',
    userInfo: JSON.parse(localStorage.getItem('userInfo') ?? 'null') as UserInfo | null,
  }),
  actions: {
    async login(payload: LoginPayload) {
      const result = await apiRequest<LoginResponse>({ url: '/api/auth/login', method: 'POST', data: payload });
      this.setSession(result);
    },
    async fetchProfile() {
      this.userInfo = await apiRequest<UserInfo>({ url: '/api/auth/profile', method: 'GET' });
      localStorage.setItem('userInfo', JSON.stringify(this.userInfo));
    },
    setSession(result: LoginResponse) {
      this.token = result.accessToken;
      this.userInfo = result.user;
      localStorage.setItem('accessToken', result.accessToken);
      localStorage.setItem('userInfo', JSON.stringify(result.user));
    },
    logout() {
      this.token = '';
      this.userInfo = null;
      localStorage.removeItem('accessToken');
      localStorage.removeItem('userInfo');
    },
  },
});
