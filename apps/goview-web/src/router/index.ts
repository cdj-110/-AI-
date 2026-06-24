import { h } from "vue";
import { createRouter, createWebHashHistory, RouterView } from "vue-router";
import useCounterStore from "@/stores/counter";
import { globalLockState } from "@/components/LockScreen/hooks/useLockScreen";

const RouterPassThrough = { render: () => h(RouterView) };

const routerList = [
  {
    path: "/",
    name: "index",
    component: () => import("@/views/index.vue"),
    meta: { title: "首页" },
    children: [
      {
        path: "/workPage",
        name: "workPage",
        component: RouterPassThrough,
        meta: { title: "工作台" },
        children: [
          {
            path: "/workPageOverview",
            name: "workPageOverview",
            component: () => import("@/views/workPage/overview/index.vue"),
            meta: { title: "概览" }
          }
        ]
      },
      {
        path: "/facility",
        name: "facility",
        component: RouterPassThrough,
        meta: { title: "设备" },
        children: [
          {
            path: "/facilityList",
            name: "facilityList",
            component: () => import("@/views/facility/facilityList/index.vue"),
            meta: { title: "设备列表" }
          },
          {
            path: "/facilityList/particularNo",
            name: "facilityList/particularNo",
            component: () => import("@/views/facility/facilityList/particularNo.vue"),
            meta: { title: "设备详情" }
          },
          {
            path: "/facilityPattern",
            name: "facilityPattern",
            component: () => import("@/views/facility/facilityPattern/index.vue"),
            meta: { title: "设备模型" }
          },
          {
            path: "/facilityPattern/modelDetails",
            name: "facilityPattern/modelDetails",
            component: () => import("@/views/facility/facilityPattern/modelDetails/index.vue"),
            meta: { title: "模型详情" }
          },
          {
            path: "/facilityGroup",
            name: "facilityGroup",
            component: () => import("@/views/facility/facilityGroup/index.vue"),
            meta: { title: "设备分组" }
          },
          {
            path: "/facilityGroup/ziGroup",
            name: "facilityGroup/ziGroup",
            component: () => import("@/views/facility/facilityGroup/ziGroup.vue"),
            meta: { title: "分组详情" }
          }
        ]
      },
      {
        path: "/warning",
        name: "warning",
        component: RouterPassThrough,
        meta: { title: "告警" },
        children: [
          {
            path: "/warningChronicle",
            name: "warningChronicle",
            component: () => import("@/views/warning/warningChronicle/index.vue"),
            meta: { title: "告警历史" }
          },
          {
            path: "/warningRule",
            name: "warningRule",
            component: () => import("@/views/warning/warningRule/index.vue"),
            meta: { title: "告警规则" }
          },
          {
            path: "/warningGroup",
            name: "warningGroup",
            component: () => import("@/views/warning/warningGroup/index.vue"),
            meta: { title: "通知组" }
          }
        ]
      },
      {
        path: "/rulesPage",
        name: "rulesPage",
        component: RouterPassThrough,
        meta: { title: "规则" },
        children: [
          {
            path: "/facilityLinkage",
            name: "facilityLinkage",
            component: () => import("@/views/rulesPage/facilityLinkage/index.vue"),
            meta: { title: "设备联动" }
          },
          {
            path: "/taskPage",
            name: "taskPage",
            component: () => import("@/views/rulesPage/taskPage/index.vue"),
            meta: { title: "定时任务" }
          }
        ]
      },
      {
        path: "/configuration",
        name: "configuration",
        component: RouterPassThrough,
        meta: { title: "云组态" },
        children: [
          {
            path: "/largeScreen",
            name: "largeScreen",
            component: () => import("@/views/configuration/largeScreen/index.vue"),
            meta: { title: "数据大屏" }
          }
        ]
      },
      {
        path: "/dataCenter",
        name: "dataCenter",
        component: RouterPassThrough,
        meta: { title: "数据中心" },
        children: [
          {
            path: "/dataReport",
            name: "dataReport",
            component: () => import("@/views/dataCenter/dataReport/index.vue"),
            meta: { title: "数据报表" }
          }
        ]
      },
      {
        path: "/userOrganize",
        name: "userOrganize",
        component: RouterPassThrough,
        meta: { title: "用户" },
        children: [
          {
            path: "/userAccount",
            name: "userAccount",
            component: () => import("@/views/userOrganize/userAccount/index.vue"),
            meta: { title: "账号管理" }
          },
          {
            path: "/userPlan",
            name: "userPlan",
            component: () => import("@/views/userOrganize/userPlan/index.vue"),
            meta: { title: "组织架构" }
          },
          {
            path: "/userRole",
            name: "userRole",
            component: () => import("@/views/userOrganize/userRole/index.vue"),
            meta: { title: "角色权限" }
          },
          {
            path: "/userLog",
            name: "userLog",
            component: () => import("@/views/userOrganize/userLog/index.vue"),
            meta: { title: "操作日志" }
          }
        ]
      }
    ]
  },
  {
    path: "/login",
    name: "login",
    component: () => import("@/views/login/login.vue"),
    meta: { title: "登录" }
  },
  {
    path: "/publicPreview/:configId",
    name: "publicPreview",
    component: () => import("@/views/configuration/largeScreen/publicPreview.vue"),
    meta: { title: "公开预览" }
  },
  {
    path: "/screenRoot:id(.*)",
    name: "screenRoot",
    component: () => import("@/views/configuration/largeScreen/screenRoot.vue"),
    meta: { title: "大屏组态" }
  }
];

const router = createRouter({
  history: createWebHashHistory(),
  routes: routerList
});

const ruleOutRouter = ["/login", "/enroll", "/publicPreview", "/chart/preview"];

router.beforeEach((to, from, next) => {
  console.info("route guard", {
    to: to.fullPath,
    from: from.fullPath,
    hasToken: !!sessionStorage.getItem("wk_Token"),
    locked: globalLockState.value
  });

  if (ruleOutRouter.some(path => to.path === path || to.path.startsWith(path + "/")) || to.path.startsWith("/screenRoot")) {
    next();
    return;
  }

  if (globalLockState.value) {
    console.warn("clearing stale lock state before navigation");
    globalLockState.value = false;
    localStorage.setItem("app_lock_screen_status", "false");
  }

  if (!sessionStorage.getItem("wk_Token")) {
    next("/login");
    return;
  }

  const useStore = useCounterStore();
  useStore.setTechnical(to.matched.map(item => ({ title: item.meta.title, path: item.path })) as any);
  next();
});

export default router;
