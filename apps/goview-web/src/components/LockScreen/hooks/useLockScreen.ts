import { ref, onMounted, onUnmounted, watch } from 'vue';

const LOCK_SCREEN_KEY = 'app_lock_screen_status';
let isEventBound = false;
let userActionHandler: () => void;
// 新增：存储锁屏前的原始路由（用于恢复）
let originalHash = window.location.hash;

export const globalLockState = ref<boolean>(
  JSON.parse(localStorage.getItem(LOCK_SCREEN_KEY) || 'false')
);

// 拦截hash变化（手动改URL的核心）
const handleHashChange = () => {
  if (globalLockState.value) {
    // 锁屏状态下，强制恢复原始hash
    window.location.hash = originalHash;
  } else {
    // 未锁屏时，更新原始hash
    originalHash = window.location.hash;
  }
};

export const useLockScreen = (lockTime: number ) => {
  const isLocked = globalLockState;
  let lockTimer: ReturnType<typeof setTimeout> | null = null;
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;

  // 监听锁屏状态：锁屏时记录当前路由，解锁时更新原始路由
  watch(isLocked, (newVal) => {
    localStorage.setItem(LOCK_SCREEN_KEY, JSON.stringify(newVal));
    if (newVal) {
      // 锁屏时记录当前路由（用于后续恢复）
      originalHash = window.location.hash;
      // 禁用右键/地址栏拖拽（增加修改难度）
    //   document.addEventListener('contextmenu', (e) => e.preventDefault());
    } else {
      // 解锁时恢复右键
      document.removeEventListener('contextmenu', (e) => e.preventDefault());
    }
  }, { immediate: true });

  const resetLockTimer = () => {
    if (isLocked.value) return;
    if (lockTimer) clearTimeout(lockTimer);
    lockTimer = setTimeout(() => {
      isLocked.value = true;
    }, lockTime);
  };

  const bindUserActionEvents = () => {
    if (isEventBound) return;
    userActionHandler = () => {
      if (debounceTimer) clearTimeout(debounceTimer);
      debounceTimer = setTimeout(() => {
        resetLockTimer();
      }, 30);
    };

    const events = ['mousemove', 'click', 'keydown', 'scroll', 'touchstart'];
    events.forEach(event => {
      window.addEventListener(event, userActionHandler);
    });

    // 新增：绑定hashchange事件，拦截手动改URL
    window.addEventListener('hashchange', handleHashChange);
    isEventBound = true;
  };

  const unbindUserActionEvents = () => {
    if (!isEventBound || !userActionHandler) return;
    const events = ['mousemove', 'click', 'keydown', 'scroll', 'touchstart'];
    events.forEach(event => {
      window.removeEventListener(event, userActionHandler);
    });

    // 新增：解绑hashchange事件
    window.removeEventListener('hashchange', handleHashChange);
    isEventBound = false;
    debounceTimer = null;
  };

  const handleUnlockSuccess = () => {
    isLocked.value = false;
    resetLockTimer();
    // 解锁时更新原始路由（防止解锁后路由异常）
    originalHash = window.location.hash;
  };

  onMounted(() => {
    resetLockTimer();
    bindUserActionEvents();
    // 初始化原始路由
    originalHash = window.location.hash;
  });

  onUnmounted(() => {
    if (lockTimer) clearTimeout(lockTimer);
    if (debounceTimer) clearTimeout(debounceTimer);
    unbindUserActionEvents();
    // 卸载时恢复右键
    document.removeEventListener('contextmenu', (e) => e.preventDefault());
  });

  return {
    isLocked,
    handleUnlockSuccess
  };
};