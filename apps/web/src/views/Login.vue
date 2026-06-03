<script setup lang="ts">
import { reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import type { FormInstance, FormRules } from 'element-plus';
import { ElMessage } from 'element-plus';
import { useUserStore } from '../stores/user';

const router = useRouter();
const userStore = useUserStore();
const formRef = ref<FormInstance>();
const loading = ref(false);
const form = reactive({ username: '', password: '', remember: true });
const rules: FormRules = {
  username: [{ required: true, message: '请输入账号', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
};

async function submit() {
  if (!(await formRef.value?.validate())) return;
  loading.value = true;
  try {
    await userStore.login(form);
    ElMessage.success('登录成功');
    router.push('/dashboard');
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="auth-page">
    <section class="auth-card">
      <div class="brand"><span class="brand-mark">微</span><span>微控物联云平台</span></div>
      <h1 class="auth-title">欢迎回来</h1>
      <p class="auth-subtitle">登录后管理您的物联网设备与数据</p>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @keyup.enter="submit">
        <el-form-item label="账号" prop="username">
          <el-input v-model="form.username" placeholder="手机号 / 邮箱 / 账号" size="large" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="form.password" type="password" placeholder="请输入密码" show-password size="large" />
        </el-form-item>
        <div class="login-options">
          <el-checkbox v-model="form.remember">记住密码</el-checkbox>
          <router-link to="/forgot-password">忘记密码？</router-link>
        </div>
        <el-button type="primary" size="large" class="full-width" :loading="loading" @click="submit">登录</el-button>
        <el-button size="large" class="full-width wechat" plain>微信快捷登录</el-button>
      </el-form>
      <p class="auth-footer">没有账号？ <router-link to="/register">立即免费注册</router-link></p>
    </section>
  </div>
</template>

<style scoped>
.login-options { display: flex; justify-content: space-between; margin: -4px 0 18px; font-size: 14px; }
.wechat { margin: 12px 0 0; }
</style>
