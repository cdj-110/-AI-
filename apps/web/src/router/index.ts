import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router';
import MainLayout from '../layouts/MainLayout.vue';
import { isAdminPortal, portalName, roleAllowed } from '../portal';
import Alarms from '../views/Alarms.vue';
import AccountSecurity from '../views/AccountSecurity.vue';
import CloudScada from '../views/CloudScada.vue';
import Dashboard from '../views/Dashboard.vue';
import DeviceDetail from '../views/DeviceDetail.vue';
import DeviceLogs from '../views/DeviceLogs.vue';
import DeviceMap from '../views/DeviceMap.vue';
import Events from '../views/Events.vue';
import Devices from '../views/Devices.vue';
import FactoryGateways from '../views/FactoryGateways.vue';
import ForgotPassword from '../views/ForgotPassword.vue';
import Login from '../views/Login.vue';
import LoginLogs from '../views/LoginLogs.vue';
import ModelTemplates from '../views/ModelTemplates.vue';
import OperationLogs from '../views/OperationLogs.vue';
import OtaPackages from '../views/OtaPackages.vue';
import PlatformEvents from '../views/PlatformEvents.vue';
import PlatformDevices from '../views/PlatformDevices.vue';
import Register from '../views/Register.vue';
import SystemStatus from '../views/SystemStatus.vue';
import Tenants from '../views/Tenants.vue';
import Users from '../views/Users.vue';
import Products from '../views/Products.vue';
import SubscriptionPlans from '../views/SubscriptionPlans.vue';
import QuotaAddons from '../views/QuotaAddons.vue';
import FinanceReviews from '../views/FinanceReviews.vue';
import MySubscription from '../views/MySubscription.vue';
import SubscriptionOrders from '../views/SubscriptionOrders.vue';
import CustomerSubscriptions from '../views/CustomerSubscriptions.vue';
import Changelog from '../views/Changelog.vue';
import OfficialHome from '../views/OfficialHome.vue';
import OfficialChangelog from '../views/OfficialChangelog.vue';
import { getAuthItem, removeAuthItem } from '../utils/auth-storage';

const adminChildren: RouteRecordRaw[] = [
  { path: 'dashboard', component: Dashboard },
  { path: 'account', component: AccountSecurity, meta: { title: '账号与安全' } },
  { path: 'system-status', component: SystemStatus, meta: { roles: ['SUPER_ADMIN'] } },
  { path: 'devices', component: PlatformDevices, meta: { roles: ['SUPER_ADMIN'] } },
  { path: 'events', component: PlatformEvents, meta: { roles: ['SUPER_ADMIN'], title: '平台事件' } },
  { path: 'factory-gateways', component: FactoryGateways, meta: { roles: ['SUPER_ADMIN'] } },
  { path: 'ota-packages', component: OtaPackages, meta: { roles: ['SUPER_ADMIN'], title: '升级包管理' } },
  { path: 'products', component: Products, meta: { roles: ['SUPER_ADMIN'] } },
  { path: 'subscription-plans', component: SubscriptionPlans, meta: { roles: ['SUPER_ADMIN'], title: '套餐管理' } },
  { path: 'quota-addons', component: QuotaAddons, meta: { roles: ['SUPER_ADMIN'], title: '扩容包管理' } },
  { path: 'subscription-orders', component: SubscriptionOrders, meta: { roles: ['SUPER_ADMIN'], title: '订单管理' } },
  { path: 'customer-subscriptions', component: CustomerSubscriptions, meta: { roles: ['SUPER_ADMIN'], title: '客户订阅' } },
  { path: 'finance-reviews', component: FinanceReviews, meta: { roles: ['SUPER_ADMIN'], title: '财务审核' } },
  { path: 'tenants', component: Tenants, meta: { roles: ['SUPER_ADMIN'] } },
  { path: 'users', component: Users, meta: { roles: ['SUPER_ADMIN'] } },
  { path: 'login-logs', component: LoginLogs, meta: { roles: ['SUPER_ADMIN'] } },
  { path: 'operation-logs', component: OperationLogs, meta: { roles: ['SUPER_ADMIN'] } },
  { path: 'device-logs', component: DeviceLogs, meta: { roles: ['SUPER_ADMIN'] } },
  { path: 'changelog-management', component: Changelog, meta: { roles: ['SUPER_ADMIN'], title: '更新日志管理', manageChangelog: true } },
];

const tenantChildren: RouteRecordRaw[] = [
  { path: 'dashboard', component: Dashboard, meta: { permission: 'DASHBOARD_VIEW' } },
  { path: 'account', component: AccountSecurity, meta: { title: '账号与安全' } },
  { path: 'devices', component: Devices, meta: { permission: 'DEVICE_VIEW' } },
  { path: 'devices/:id', component: DeviceDetail, meta: { permission: 'DEVICE_VIEW' } },
  { path: 'device-map', component: DeviceMap, meta: { permission: 'MAP_VIEW' } },
  { path: 'data-center', component: () => import('../views/DataCenter.vue'), meta: { permission:'DEVICE_VIEW', title:'数据中心' } },
  { path: 'device-linkage', component: () => import('../views/DeviceLinkage.vue'), meta: { roles: ['TENANT_ADMIN'], title: '设备联动' } },
  { path: 'data-center/:id', component: () => import('../views/HistoryTaskDetail.vue'), meta: { permission:'DEVICE_VIEW', title:'历史数据任务' } },
  { path: 'cloud-scada', component: CloudScada, meta: { permission: 'SCADA_VIEW' } },
  { path: 'model-templates', component: ModelTemplates, meta: { permission: 'MODEL_VIEW' } },
  { path: 'alarms', component: Alarms, meta: { permission: 'ALARM_VIEW' } },
  { path: 'events', component: Events, meta: { permission: 'EVENT_VIEW' } },
  { path: 'login-logs', component: LoginLogs, meta: { roles: ['TENANT_ADMIN'] } },
  { path: 'operation-logs', component: OperationLogs, meta: { roles: ['TENANT_ADMIN'] } },
  { path: 'device-logs', component: DeviceLogs, meta: { roles: ['TENANT_ADMIN'] } },
  { path: 'users', component: Users, meta: { roles: ['TENANT_ADMIN'] } },
  { path: 'subscription', component: MySubscription, meta: { roles: ['TENANT_ADMIN'], title: '版本与升级' } },
];

const guestRoutes: RouteRecordRaw[] = [
  { path: '/session-login', component: Login, meta: { title: '重新登录' } },
  { path: '/login', component: Login, meta: { guestOnly: true } },
  ...(!isAdminPortal ? [
    { path: '/', component: OfficialHome, meta: { title: '工业物联网云平台' } },
    { path: '/changelog', component: OfficialChangelog, meta: { title: '更新日志' } },
    { path: '/register', component: Register, meta: { guestOnly: true } },
    { path: '/forgot-password', component: ForgotPassword, meta: { guestOnly: true } },
  ] as RouteRecordRaw[] : []),
];

const router = createRouter({
  history: createWebHistory(),
  routes: [
    ...guestRoutes,
    ...(!isAdminPortal ? [{
      path: '/remote-terminal/:id', component: () => import('../views/RemoteTerminal.vue'),
      meta: { requiresAuth: true, roles: ['TENANT_ADMIN'], title: '远程 SSH 终端' },
    } as RouteRecordRaw] : []),
    {
      path: '/console',
      component: MainLayout,
      meta: { requiresAuth: true },
      redirect: '/dashboard',
      children: (isAdminPortal ? adminChildren : tenantChildren).map(item => ({ ...item, path: `/${item.path}` })),
    },
    { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
  ],
});

router.beforeEach((to) => {
  if (to.path === '/session-login') return;
  const token = getAuthItem('accessToken');
  const userInfo = JSON.parse(getAuthItem('userInfo') ?? 'null') as { role?: string; permissions?: string[] } | null;
  if (token && !roleAllowed(userInfo?.role)) {
    removeAuthItem('accessToken');
    removeAuthItem('userInfo');
    return { path: '/login', query: { reason: 'portal' } };
  }
  if (to.meta.requiresAuth && !token) return '/login';
  if (to.meta.guestOnly && token) return '/dashboard';
  if (to.meta.roles && (!userInfo?.role || !(to.meta.roles as string[]).includes(userInfo.role))) return '/dashboard';
  if (to.meta.permission && userInfo?.role === 'TENANT_USER' && Array.isArray(userInfo.permissions) && !userInfo.permissions.includes(String(to.meta.permission))) {
    const destinations = [
      ['DASHBOARD_VIEW', '/dashboard'], ['DEVICE_VIEW', '/devices'], ['MAP_VIEW', '/device-map'],
      ['SCADA_VIEW', '/cloud-scada'], ['MODEL_VIEW', '/model-templates'], ['ALARM_VIEW', '/alarms'], ['EVENT_VIEW', '/events'],
    ];
    return destinations.find(([permission]) => userInfo.permissions?.includes(permission))?.[1] ?? '/account';
  }
});

router.afterEach((to) => {
  document.title = `${String(to.meta.title || '控制台')} - ${portalName}`;
});

export default router;
