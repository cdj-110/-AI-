import { createRouter, createWebHistory } from 'vue-router';
import MainLayout from '../layouts/MainLayout.vue';
import Alarms from '../views/Alarms.vue';
import Dashboard from '../views/Dashboard.vue';
import DeviceLogs from '../views/DeviceLogs.vue';
import Devices from '../views/Devices.vue';
import DeviceDetail from '../views/DeviceDetail.vue';
import DeviceMap from '../views/DeviceMap.vue';
import ForgotPassword from '../views/ForgotPassword.vue';
import Login from '../views/Login.vue';
import LoginLogs from '../views/LoginLogs.vue';
import ModelTemplates from '../views/ModelTemplates.vue';
import OperationLogs from '../views/OperationLogs.vue';
import Register from '../views/Register.vue';
import SystemStatus from '../views/SystemStatus.vue';
import Tenants from '../views/Tenants.vue';
import Users from '../views/Users.vue';

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login, meta: { guestOnly: true } },
    { path: '/register', component: Register, meta: { guestOnly: true } },
    { path: '/forgot-password', component: ForgotPassword, meta: { guestOnly: true } },
    {
      path: '/',
      component: MainLayout,
      meta: { requiresAuth: true },
      children: [
        { path: '', redirect: '/dashboard' },
        { path: 'dashboard', component: Dashboard },
        { path: 'system-status', component: SystemStatus, meta: { roles: ['SUPER_ADMIN', 'TENANT_ADMIN'] } },
        { path: 'devices', component: Devices },
        { path: 'devices/:id', component: DeviceDetail },
        { path: 'device-map', component: DeviceMap },
        { path: 'model-templates', component: ModelTemplates },
        { path: 'alarms', component: Alarms },
        { path: 'login-logs', component: LoginLogs, meta: { roles: ['SUPER_ADMIN', 'TENANT_ADMIN'] } },
        { path: 'operation-logs', component: OperationLogs, meta: { roles: ['SUPER_ADMIN', 'TENANT_ADMIN'] } },
        { path: 'device-logs', component: DeviceLogs, meta: { roles: ['SUPER_ADMIN', 'TENANT_ADMIN'] } },
        { path: 'users', component: Users, meta: { roles: ['SUPER_ADMIN', 'TENANT_ADMIN'] } },
        { path: 'tenants', component: Tenants, meta: { roles: ['SUPER_ADMIN'] } },
      ],
    },
  ],
});

router.beforeEach((to) => {
  // 前端路由只做体验层拦截，真实权限仍以后端 JWT + RolesGuard 为准。
  const token = localStorage.getItem('accessToken');
  const userInfo = JSON.parse(localStorage.getItem('userInfo') ?? 'null') as { role?: string } | null;
  if (to.meta.requiresAuth && !token) return '/login';
  if (to.meta.guestOnly && token) return '/dashboard';
  if (to.meta.roles && (!userInfo?.role || !(to.meta.roles as string[]).includes(userInfo.role))) return '/dashboard';
});

export default router;
