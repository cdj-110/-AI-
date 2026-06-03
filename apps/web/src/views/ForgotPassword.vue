<script setup lang="ts">
import { reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import type { FormInstance, FormRules } from 'element-plus';
import { ElMessage } from 'element-plus';
import { apiRequest } from '../api/request';

const router = useRouter();
const formRef = ref<FormInstance>();
const loading = ref(false);
const form = reactive({ username: '', code: '', password: '', confirmPassword: '' });
const rules: FormRules = {
  username: [{ required: true, message: '请输入账号', trigger: 'blur' }],
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
    await apiRequest({ url: '/api/auth/forgot-password/reset', method: 'POST', data: form });
    ElMessage.success('密码已重置，请重新登录');
    router.push('/login');
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="auth-page">
    <section class="auth-card">
      <div class="brand"><span class="brand-mark">微</span><span>微控物联云平台</span></div>
      <h1 class="auth-title">重置密码</h1>
      <p class="auth-subtitle">验证账号后设置一个新的登录密码</p>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="账号" prop="username"><el-input v-model="form.username" size="large" /></el-form-item>
        <el-form-item label="验证码" prop="code"><el-input v-model="form.code" placeholder="演示验证码：123456" size="large" /></el-form-item>
        <el-form-item label="新密码" prop="password"><el-input v-model="form.password" type="password" show-password size="large" /></el-form-item>
        <el-form-item label="确认新密码" prop="confirmPassword"><el-input v-model="form.confirmPassword" type="password" show-password size="large" /></el-form-item>
        <el-button type="primary" size="large" class="full-width" :loading="loading" @click="submit">提交</el-button>
      </el-form>
      <p class="auth-footer"><router-link to="/login">返回登录</router-link></p>
    </section>
  </div>
</template>
