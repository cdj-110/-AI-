<script setup lang="ts">
import { reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import type { FormInstance, FormRules } from 'element-plus';
import { ElMessage } from 'element-plus';
import { apiRequest } from '../api/request';
import { useUserStore, type UserInfo } from '../stores/user';

const router = useRouter();
const userStore = useUserStore();
const formRef = ref<FormInstance>();
const loading = ref(false);
const mode = ref('phone');
const form = reactive({ username: '', phone: '', email: '', code: '', password: '', confirmPassword: '' });
const rules: FormRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  code: [{ required: true, message: '请输入验证码', trigger: 'blur' }],
  password: [{ required: true, min: 6, message: '密码至少 6 位', trigger: 'blur' }],
  confirmPassword: [{
    validator: (_rule, value, callback) => value === form.password ? callback() : callback(new Error('两次输入的密码不一致')),
    trigger: 'blur',
  }],
};

async function submit() {
  if (!(await formRef.value?.validate())) return;
  loading.value = true;
  try {
    const result = await apiRequest<{ accessToken: string; user: UserInfo }>({
      url: '/api/auth/register',
      method: 'POST',
      data: { ...form, phone: mode.value === 'phone' ? form.phone : undefined, email: mode.value === 'email' ? form.email : undefined },
    });
    userStore.setSession(result);
    ElMessage.success('注册成功');
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
      <h1 class="auth-title">创建账号</h1>
      <p class="auth-subtitle">开始搭建您的设备管理空间</p>
      <el-tabs v-model="mode" stretch>
        <el-tab-pane label="手机号注册" name="phone" />
        <el-tab-pane label="邮箱注册" name="email" />
      </el-tabs>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="用户名" prop="username"><el-input v-model="form.username" size="large" /></el-form-item>
        <el-form-item v-if="mode === 'phone'" label="手机号"><el-input v-model="form.phone" size="large" /></el-form-item>
        <el-form-item v-else label="邮箱"><el-input v-model="form.email" size="large" /></el-form-item>
        <el-form-item label="验证码" prop="code"><el-input v-model="form.code" placeholder="演示验证码：123456" size="large" /></el-form-item>
        <el-form-item label="密码" prop="password"><el-input v-model="form.password" type="password" show-password size="large" /></el-form-item>
        <el-form-item label="确认密码" prop="confirmPassword"><el-input v-model="form.confirmPassword" type="password" show-password size="large" /></el-form-item>
        <el-button type="primary" size="large" class="full-width" :loading="loading" @click="submit">注册</el-button>
      </el-form>
      <p class="auth-footer">已有账号？ <router-link to="/login">返回登录</router-link></p>
    </section>
  </div>
</template>
