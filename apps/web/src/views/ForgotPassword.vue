<script setup lang="ts">
import { onBeforeUnmount, reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import type { FormInstance, FormRules } from 'element-plus';
import { ElMessage } from 'element-plus';
import tenantPlatformLogo from '../assets/tenant-platform-logo.png';
import { apiRequest } from '../api/request';

const router = useRouter();
const formRef = ref<FormInstance>();
const loading = ref(false);
const sendingCode = ref(false);
const countdown = ref(0);
let countdownTimer: number | undefined;
const form = reactive({ username: '', code: '', password: '', confirmPassword: '' });
const rules: FormRules = {
  username: [{ required: true, message: '请输入邮箱或手机号', trigger: 'blur' }],
  code: [{ required: true, message: '请输入验证码', trigger: 'blur' }],
  password: [{ required: true, min: 6, message: '密码至少 6 位', trigger: 'blur' }],
  confirmPassword: [{
    validator: (_rule, value, callback) => value === form.password ? callback() : callback(new Error('两次输入的密码不一致')),
    trigger: 'blur',
  }],
};

async function sendCode() {
  try {
    await formRef.value?.validateField('username');
  } catch {
    return;
  }
  sendingCode.value = true;
  try {
    const result = await apiRequest<{ email: string; retryAfterSeconds: number }>({
      url: '/api/auth/email-code',
      method: 'POST',
      timeout: 20000,
      data: { purpose: 'reset', target: form.username },
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
    await apiRequest({ url: '/api/auth/forgot-password/reset', method: 'POST', data: form });
    ElMessage.success('密码已重置，请重新登录');
    router.push('/login');
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
      <h1 class="auth-title">重置密码</h1>
      <p class="auth-subtitle">验证登录邮箱或手机号后设置新密码</p>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="邮箱或手机号" prop="username"><el-input v-model.trim="form.username" autocomplete="username" placeholder="请输入登录邮箱或手机号" size="large" /></el-form-item>
        <el-form-item label="邮箱验证码" prop="code">
          <div class="code-row">
            <el-input v-model.trim="form.code" maxlength="6" inputmode="numeric" placeholder="请输入 6 位验证码" size="large" />
            <el-button size="large" :loading="sendingCode" :disabled="countdown > 0" @click="sendCode">
              {{ countdown > 0 ? `${countdown} 秒后重发` : '获取验证码' }}
            </el-button>
          </div>
        </el-form-item>
        <el-form-item label="新密码" prop="password"><el-input v-model="form.password" type="password" show-password size="large" /></el-form-item>
        <el-form-item label="确认新密码" prop="confirmPassword"><el-input v-model="form.confirmPassword" type="password" show-password size="large" /></el-form-item>
        <el-button type="primary" size="large" class="full-width" :loading="loading" @click="submit">提交</el-button>
      </el-form>
      <p class="auth-footer"><router-link to="/login">返回登录</router-link></p>
    </section>
  </div>
</template>

<style scoped>
.tenant-brand-logo { display: block; box-sizing: border-box; width: 38px; height: 38px; padding: 2px; border: 0; object-fit: contain; background: transparent; }
.code-row { display: grid; grid-template-columns: minmax(0, 1fr) 132px; gap: 10px; width: 100%; }
</style>
