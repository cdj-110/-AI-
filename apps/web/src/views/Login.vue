<script setup lang="ts">
import { computed, nextTick, onMounted, onBeforeUnmount, reactive, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import type { FormInstance, FormRules } from 'element-plus';
import { ElMessage } from 'element-plus';
import { useUserStore } from '../stores/user';
import { alternatePortalURL, isAdminPortal, portalLogo, portalName } from '../portal';
import { rememberedUsername, rememberLoginEnabled } from '../utils/auth-storage';
import SliderCaptcha from '../components/SliderCaptcha.vue';
import { initSessionLogin, type RecoveryIdentity } from '../utils/session-login';

const router = useRouter();
const route = useRoute();
const userStore = useUserStore();
const formRef = ref<FormInstance>();
const captchaRef = ref<InstanceType<typeof SliderCaptcha>>();
const loading = ref(false);
const sendingCode = ref(false);
const countdown = ref(0);
const loginMode = ref<'password' | 'code'>('password');
const captchaAction = ref<'login' | 'sendCode'>('login');
const captchaDialogVisible = ref(false);
const form = reactive({ username: rememberedUsername(), password: '', code: '', captchaToken: '', remember: rememberLoginEnabled() });
const rules = computed<FormRules>(() => ({
  username: [{ required: true, message: '请输入邮箱或手机号', trigger: 'blur' }],
  password: loginMode.value === 'password' ? [{ required: true, message: '请输入密码', trigger: 'blur' }] : [],
  code: loginMode.value === 'code' ? [
    { required: true, message: '请输入验证码', trigger: 'blur' },
    { pattern: /^\d{6}$/, message: '请输入 6 位数字验证码', trigger: 'blur' },
  ] : [],
}));
const platformMismatch = route.query.reason === 'portal';
const embedded = route.path === '/session-login';
const recoveryIdentity = ref<RecoveryIdentity>();
const cardRef = ref<HTMLElement>();
let cardObserver: ResizeObserver | undefined;
let recoveryBridge: ReturnType<typeof initSessionLogin> | undefined;
onMounted(() => {
  if (embedded) {
    recoveryBridge = initSessionLogin(identity => {
      recoveryIdentity.value = identity;
      void nextTick(() => recoveryBridge?.reportHeight(cardRef.value?.offsetHeight || 680));
    });
    cardObserver = new ResizeObserver(() => recoveryBridge?.reportHeight(cardRef.value?.offsetHeight || 680));
    if (cardRef.value) cardObserver.observe(cardRef.value);
  }
});
let countdownTimer: number | undefined;

async function submit() {
  if (loading.value || (embedded && !recoveryIdentity.value)) return;
  if (!(await formRef.value?.validate())) return;
  loading.value = true;
  try {
    const payload = { ...form, recoveryIdentity: recoveryIdentity.value };
    if (loginMode.value === 'code') await userStore.loginWithCode(payload);
    else await userStore.login(payload);
    ElMessage.success('登录成功');
    if (embedded) recoveryBridge?.send(userStore.token);
    else router.push('/dashboard');
  } catch (error) {
    if (embedded && !(error as { response?: unknown }).response) ElMessage.error((error as Error).message);
    form.captchaToken = '';
    const status = (error as { response?: { status?: number } }).response?.status;
    if (status === 428 && loginMode.value === 'password') {
      captchaAction.value = 'login';
      captchaDialogVisible.value = true;
    }
  } finally {
    loading.value = false;
  }
}

async function captchaVerified(token: string) {
  if (!token) return;
  await new Promise((resolve) => window.setTimeout(resolve, 1200));
  captchaDialogVisible.value = false;
  if (captchaAction.value === 'sendCode') {
    await sendLoginCode(token);
    return;
  }
  form.captchaToken = token;
  await submit();
}

function requestLoginCode() {
  if (!form.username.trim()) {
    ElMessage.warning('请先输入邮箱或手机号');
    return;
  }
  captchaAction.value = 'sendCode';
  captchaDialogVisible.value = true;
}

async function sendLoginCode(captchaToken: string) {
  sendingCode.value = true;
  try {
    const result = await userStore.sendLoginCode({ username: form.username, captchaToken });
    ElMessage.success(result.email ? `验证码已发送至 ${result.email}` : result.message);
    startCountdown(result.retryAfterSeconds || 60);
  } finally {
    sendingCode.value = false;
    captchaRef.value?.reset();
  }
}

function startCountdown(seconds: number) {
  if (countdownTimer) window.clearInterval(countdownTimer);
  countdown.value = Math.max(1, Math.floor(seconds));
  countdownTimer = window.setInterval(() => {
    countdown.value -= 1;
    if (countdown.value <= 0 && countdownTimer) {
      window.clearInterval(countdownTimer);
      countdownTimer = undefined;
    }
  }, 1000);
}

watch(() => [form.username, form.password, loginMode.value], () => {
  if (!form.captchaToken) return;
  form.captchaToken = '';
  captchaRef.value?.reset();
});

onBeforeUnmount(() => {
  recoveryBridge?.dispose();
  cardObserver?.disconnect();
  if (countdownTimer) window.clearInterval(countdownTimer);
});
</script>

<template>
  <div class="auth-page" :class="{ 'embedded-login': embedded }">
    <section ref="cardRef" class="auth-card">
      <div class="brand"><img class="portal-brand-logo" :class="{ admin: isAdminPortal }" :src="portalLogo" :alt="portalName" /><span>{{ portalName }}</span></div>
      <h1 class="auth-title">{{ isAdminPortal ? '公司管理登录' : '租户登录' }}</h1>
      <p class="auth-subtitle">{{ isAdminPortal ? '用于租户、网关和平台运行管理' : '登录后管理本租户的设备与数据' }}</p>
      <el-alert v-if="platformMismatch" class="portal-alert" type="warning" :closable="false" show-icon title="当前账号不属于这个平台，请使用正确入口登录" />
      <el-segmented v-model="loginMode" class="login-mode" :options="[{ label: '密码登录', value: 'password' }, { label: '验证码登录', value: 'code' }]" />
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" autocomplete="on" @keyup.enter="submit">
        <el-form-item label="邮箱或手机号" prop="username">
          <el-input v-model.trim="form.username" name="username" autocomplete="username" placeholder="请输入邮箱或手机号" size="large" />
        </el-form-item>
        <el-form-item v-if="loginMode === 'password'" label="密码" prop="password">
          <el-input v-model="form.password" name="password" autocomplete="current-password" type="password" placeholder="请输入密码" show-password size="large" />
        </el-form-item>
        <el-form-item v-else label="邮箱验证码" prop="code">
          <div class="code-input-row">
            <el-input v-model.trim="form.code" name="code" autocomplete="one-time-code" maxlength="6" inputmode="numeric" placeholder="请输入 6 位验证码" size="large" />
            <el-button size="large" :loading="sendingCode" :disabled="countdown > 0" @click="requestLoginCode">
              {{ countdown > 0 ? `${countdown} 秒后重发` : '获取验证码' }}
            </el-button>
          </div>
          <p class="code-hint">验证码将发送至该账号绑定的邮箱</p>
        </el-form-item>
        <div class="login-options">
          <el-checkbox v-model="form.remember" title="仅在此浏览器中保存邮箱或手机号，不保存密码">记住登录凭证</el-checkbox>
          <a v-if="!isAdminPortal && embedded" href="/forgot-password" target="_blank" rel="noopener">忘记密码？</a>
          <router-link v-else-if="!isAdminPortal" to="/forgot-password">忘记密码？</router-link>
        </div>
        <el-button type="primary" size="large" class="full-width" :loading="loading" @click="submit">登录</el-button>
        <el-button v-if="!isAdminPortal" size="large" class="full-width wechat" plain>微信快捷登录</el-button>
      </el-form>
      <p v-if="!isAdminPortal" class="auth-footer">没有账号？ <a v-if="embedded" href="/register" target="_blank" rel="noopener">立即免费注册</a><router-link v-else to="/register">立即免费注册</router-link></p>
      <p class="portal-switch">需要进入{{ isAdminPortal ? '物联网平台' : '公司管理平台' }}？ <a :href="alternatePortalURL" :target="embedded ? '_blank' : undefined" rel="noopener">切换入口</a></p>
    </section>
    <el-dialog
      v-model="captchaDialogVisible"
      class="captcha-dialog"
      width="390px"
      title="完成安全验证"
      destroy-on-close
      append-to-body
      :close-on-click-modal="false"
    >
      <p class="captcha-dialog-hint">拖动滑块，将拼图移动到缺口位置</p>
      <SliderCaptcha ref="captchaRef" @verified="captchaVerified" />
    </el-dialog>
  </div>
</template>

<style scoped>
/* 与网关独立登录页共用视觉背景；嵌入式重新登录卡片保持白底。 */
.auth-page:not(.embedded-login) {
  background: radial-gradient(circle at 20% 10%, #dbeafe 0, transparent 32%), linear-gradient(135deg, #f8fbff, #eef4fb);
}
.auth-page.embedded-login { padding: 0; min-height: 0; background: #fff; }
.embedded-login .auth-card { width: 100%; border: 0; box-shadow: none; border-radius: 0; }
.login-options { display: flex; justify-content: space-between; margin: -4px 0 18px; font-size: 14px; }
.login-mode { display: flex; width: 100%; margin: -4px 0 22px; }
.code-input-row { display: grid; grid-template-columns: minmax(0, 1fr) 128px; gap: 10px; width: 100%; }
.code-hint { width: 100%; margin: 8px 0 0; color: #8b96a8; font-size: 12px; }
.portal-brand-logo { display: block; box-sizing: border-box; width: 38px; height: 38px; padding: 2px; border: 0; object-fit: contain; background: transparent; }
.portal-brand-logo.admin { width: 52px; padding: 1px; border-radius: 8px; background: #fff; }
.wechat { margin: 12px 0 0; }
.portal-alert { margin: -12px 0 20px; }
.portal-switch { margin: 18px 0 0; color: #8b96a8; font-size: 13px; text-align: center; }
.captcha-dialog-hint { margin: -6px 0 14px; color: #7b8799; font-size: 13px; }
:global(.captcha-dialog) { max-width: calc(100vw - 32px); border-radius: 14px; }
:global(.captcha-dialog .el-dialog__header) { margin-right: 0; padding-bottom: 14px; border-bottom: 1px solid #edf1f6; }
:global(.captcha-dialog .el-dialog__body) { padding-top: 20px; }
</style>
