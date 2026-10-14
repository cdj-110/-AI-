<script setup lang="ts">
import { onBeforeUnmount, reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import type { FormInstance, FormRules } from 'element-plus';
import { ElMessage } from 'element-plus';
import tenantPlatformLogo from '../assets/tenant-platform-logo.png';
import { apiRequest } from '../api/request';
import { useUserStore, type LoginResponse } from '../stores/user';

const router = useRouter();
const userStore = useUserStore();
const formRef = ref<FormInstance>();
const loading = ref(false);
const sendingCode = ref(false);
const countdown = ref(0);
let countdownTimer: number | undefined;
const form = reactive({ username: '', email: '', code: '', password: '', confirmPassword: '' });
const rules: FormRules = {
  username: [{ required: true, message: '请输入用户名称', trigger: 'blur' }],
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '请输入正确的邮箱地址', trigger: ['blur', 'change'] },
  ],
  code: [{ required: true, message: '请输入验证码', trigger: 'blur' }],
  password: [{ required: true, min: 6, message: '密码至少 6 位', trigger: 'blur' }],
  confirmPassword: [{
    validator: (_rule, value, callback) => value === form.password ? callback() : callback(new Error('两次输入的密码不一致')),
    trigger: 'blur',
  }],
};

async function sendCode() {
  try {
    await formRef.value?.validateField('email');
  } catch {
    return;
  }
  sendingCode.value = true;
  try {
    const result = await apiRequest<{ email: string; retryAfterSeconds: number }>({
      url: '/api/auth/email-code',
      method: 'POST',
      timeout: 20000,
      data: { purpose: 'register', target: form.email },
    });
    ElMessage.success(`验证码已发送至 ${result.email}`);
    countdown.value = result.retryAfterSeconds;
    window.clearInterval(countdownTimer);
    countdownTimer = window.setInterval(() => {
      countdown.value -= 1;
      if (countdown.value <= 0) window.clearInterval(countdownTimer);
    }, 1000);
  } finally {
    sendingCode.value = false;
  }
}

async function submit() {
  if (!(await formRef.value?.validate())) return;
  loading.value = true;
  try {
    const result = await apiRequest<LoginResponse>({
      url: '/api/auth/register',
      method: 'POST',
      data: form,
    });
    userStore.setSession(result);
    ElMessage.success('注册成功');
    router.push('/dashboard');
  } finally {
    loading.value = false;
  }
}

onBeforeUnmount(() => window.clearInterval(countdownTimer));
</script>

<template>
  <div class="auth-page">
    <section class="auth-card">
      <div class="brand"><img class="tenant-brand-logo" :src="tenantPlatformLogo" alt="微控物联网平台" /><span>微控物联云平台</span></div>
      <h1 class="auth-title">创建账号</h1>
      <p class="auth-subtitle">开始搭建您的设备管理空间</p>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="用户名称" prop="username"><el-input v-model.trim="form.username" placeholder="用于展示，可与其他用户重复" size="large" /></el-form-item>
        <el-form-item label="登录邮箱" prop="email"><el-input v-model.trim="form.email" autocomplete="email" placeholder="注册后使用该邮箱登录" size="large" /></el-form-item>
        <el-form-item label="邮箱验证码" prop="code">
          <div class="code-row">
            <el-input v-model.trim="form.code" maxlength="6" inputmode="numeric" placeholder="请输入 6 位验证码" size="large" />
            <el-button size="large" :loading="sendingCode" :disabled="countdown > 0" @click="sendCode">
              {{ countdown > 0 ? `${countdown} 秒后重发` : '获取验证码' }}
            </el-button>
          </div>
        </el-form-item>
        <el-form-item label="密码" prop="password"><el-input v-model="form.password" type="password" show-password size="large" /></el-form-item>
        <el-form-item label="确认密码" prop="confirmPassword"><el-input v-model="form.confirmPassword" type="password" show-password size="large" /></el-form-item>
        <el-button type="primary" size="large" class="full-width" :loading="loading" @click="submit">注册</el-button>
      </el-form>
      <p class="auth-footer">已有账号？ <router-link to="/login">返回登录</router-link></p>
    </section>
  </div>
</template>

<style scoped>
.tenant-brand-logo { display: block; box-sizing: border-box; width: 38px; height: 38px; padding: 2px; border: 0; object-fit: contain; background: transparent; }
.code-row { display: grid; grid-template-columns: minmax(0, 1fr) 132px; gap: 10px; width: 100%; }
</style>
