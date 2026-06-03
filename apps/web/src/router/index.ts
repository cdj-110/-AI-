import { createRouter, createWebHistory } from 'vue-router';
import MainLayout from '../layouts/MainLayout.vue';
import Alarms from '../views/Alarms.vue';
import Dashboard from '../views/Dashboard.vue';
import Devices from '../views/Devices.vue';
import DeviceDetail from '../views/DeviceDetail.vue';
import ForgotPassword from '../views/ForgotPassword.vue';
import Login from '../views/Login.vue';
import Register from '../views/Register.vue';
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
        { path: 'devices', component: Devices },
        { path: 'devices/:id', component: DeviceDetail },
        { path: 'alarms', component: Alarms },
        { path: 'users', component: Users, meta: { roles: ['SUPER_ADMIN', 'TENANT_ADMIN'] } },
        { path: 'tenants', component: Tenants, meta: { roles: ['SUPER_ADMIN'] } },
      ],
    },
  ],
});

router.beforeEach((to) => {
  const token = localStorage.getItem('accessToken');
  const userInfo = JSON.parse(localStorage.getItem('userInfo') ?? 'null') as { role?: string } | null;
  if (to.meta.requiresAuth && !token) return '/login';
  if (to.meta.guestOnly && token) return '/dashboard';
  if (to.meta.roles && (!userInfo?.role || !(to.meta.roles as string[]).includes(userInfo.role))) return '/dashboard';
});

export default router;
