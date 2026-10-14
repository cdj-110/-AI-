<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { apiRequest } from '../api/request';
import { type ProfileUpdateResponse, useUserStore } from '../stores/user';
import { userDisplayName } from '../utils/user-display-name';

type ContactType = 'email' | 'phone';

const router = useRouter();
const userStore = useUserStore();
const loading = ref(false);
const dialogVisible = ref(false);
const sendingCode = ref(false);
const saving = ref(false);
const countdown = ref(0);
const verificationTarget = ref('');
const avatarInput = ref<HTMLInputElement>();
const avatarUploading = ref(false);
const profileSaving = ref(false);
const profileName = ref('');
const nameEditing = ref(false);
const form = reactive({ type: 'email' as ContactType, value: '', code: '' });
let countdownTimer: number | undefined;

const user = computed(() => userStore.userInfo);
const displayName = computed(() => userDisplayName(user.value));
const dialogTitle = computed(() => `${user.value?.[form.type] ? '更换' : '绑定'}${form.type === 'email' ? '邮箱' : '手机号'}`);
const verificationDescription = computed(() => form.type === 'email'
  ? '验证码将发送到新的邮箱，用于确认该邮箱归您所有。'
  : `验证码将发送到当前绑定邮箱${user.value?.email ? `（${user.value.email}）` : ''}，用于确认本次修改。`);

function roleLabel(role?: string) {
  return { SUPER_ADMIN: '公司管理员', TENANT_ADMIN: '租户管理员', TENANT_USER: '租户成员' }[role || ''] || role || '-';
}

function chooseAvatar() {
  avatarInput.value?.click();
}

function startEditingName() {
  profileName.value = displayName.value;
  nameEditing.value = true;
}

function cancelEditingName() {
  profileName.value = displayName.value;
  nameEditing.value = false;
}

async function uploadAvatar(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = '';
  if (!file) return;
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
    ElMessage.warning('请选择 JPG、PNG 或 WebP 图片');
    return;
  }
  if (file.size > 2 * 1024 * 1024) {
    ElMessage.warning('头像图片不能超过 2 MB');
    return;
  }
  const data = new FormData();
  data.append('avatar', file);
  avatarUploading.value = true;
  try {
    const result = await apiRequest<ProfileUpdateResponse>({
      url: '/api/auth/account/avatar',
      method: 'POST',
      data,
      timeout: 20000,
    });
    userStore.applyProfileUpdate(result);
    ElMessage.success('头像已更新');
  } finally {
    avatarUploading.value = false;
  }
}

async function saveProfileName() {
  const username = profileName.value.trim();
  if (!username) {
    ElMessage.warning('请输入用户名称');
    return;
  }
  if (username.length > 50) {
    ElMessage.warning('用户名称不能超过 50 个字符');
    return;
  }
  if (username === displayName.value) return;
  profileSaving.value = true;
  try {
    const result = await apiRequest<ProfileUpdateResponse>({
      url: '/api/auth/account/profile',
      method: 'PATCH',
      data: { username },
    });
    userStore.applyProfileUpdate(result);
    profileName.value = userDisplayName(result.user);
    nameEditing.value = false;
    ElMessage.success('用户名称已更新');
  } finally {
    profileSaving.value = false;
  }
}

function openContactDialog(type: ContactType) {
  if (type === 'phone' && !user.value?.email) {
    ElMessage.warning('请先绑定邮箱，再绑定或修改手机号');
    return;
  }
  form.type = type;
  form.value = user.value?.[type] || '';
  form.code = '';
  verificationTarget.value = '';
  countdown.value = 0;
  window.clearInterval(countdownTimer);
  dialogVisible.value = true;
}

function validateValue() {
  const value = form.value.trim();
  if (form.type === 'email' && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) {
    ElMessage.warning('请输入正确的邮箱地址');
    return false;
  }
  if (form.type === 'phone' && !/^\+?[\d\s-]{6,20}$/.test(value)) {
    ElMessage.warning('请输入正确的手机号');
    return false;
  }
  return true;
}

async function sendCode() {
  if (!validateValue()) return;
  sendingCode.value = true;
  try {
    const result = await apiRequest<{ email: string; retryAfterSeconds: number; message: string }>({
      url: '/api/auth/account/contact-code',
      method: 'POST',
      timeout: 20000,
      data: { type: form.type, value: form.value },
    });
    verificationTarget.value = result.email;
    ElMessage.success(`${result.message}：${result.email}`);
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

async function saveContact() {
  if (!validateValue()) return;
  if (!/^\d{6}$/.test(form.code.trim())) {
    ElMessage.warning('请输入 6 位验证码');
    return;
  }
  saving.value = true;
  try {
    await apiRequest({
      url: '/api/auth/account/contact',
      method: 'PATCH',
      data: { type: form.type, value: form.value, code: form.code },
    });
    ElMessage.success('登录凭证已更新，请使用新凭证重新登录');
    await userStore.logout(false);
    router.replace('/login');
  } finally {
    saving.value = false;
  }
}

onMounted(async () => {
  loading.value = true;
  try {
    await userStore.fetchProfile();
    profileName.value = userDisplayName(user.value);
  } finally {
    loading.value = false;
  }
});

onBeforeUnmount(() => window.clearInterval(countdownTimer));
</script>

<template>
  <div v-loading="loading" class="account-page">
    <div class="page-actions">
      <div>
        <h1 class="page-title">账号与安全</h1>
        <p class="page-description">维护当前登录账号的身份信息和登录凭证</p>
      </div>
    </div>

    <section class="account-panel identity-panel">
      <div class="avatar-editor">
        <button v-loading="avatarUploading" type="button" class="avatar-large" aria-label="修改头像" @click="chooseAvatar">
          <img v-if="user?.avatarUrl" :src="user.avatarUrl" alt="用户头像" />
          <span v-else>{{ displayName.slice(0, 1) }}</span>
          <em>修改头像</em>
        </button>
        <input ref="avatarInput" class="avatar-input" type="file" accept="image/jpeg,image/png,image/webp" @change="uploadAvatar" />
        <small>JPG、PNG、WebP，最大 2 MB</small>
      </div>
      <div class="identity-content">
        <div class="identity-title">
          <div><h2>基本资料</h2><p>用户名称用于个人展示，邮箱和手机号在下方单独维护</p></div>
          <el-tag effect="plain">{{ roleLabel(user?.role) }}</el-tag>
        </div>
        <div v-if="!nameEditing" class="name-readonly">
          <span>用户名称：{{ displayName }}</span>
          <el-button type="primary" plain @click="startEditingName">编辑</el-button>
        </div>
        <div v-else class="name-editor">
          <el-input v-model="profileName" maxlength="50" show-word-limit placeholder="请输入用户名称" autofocus @keyup.enter="saveProfileName" @keyup.esc="cancelEditingName" />
          <el-button :disabled="profileSaving" @click="cancelEditingName">取消</el-button>
          <el-button type="primary" :loading="profileSaving" :disabled="!profileName.trim() || profileName.trim() === displayName" @click="saveProfileName">保存</el-button>
        </div>
      </div>
    </section>

    <section class="account-panel">
      <div class="section-heading">
        <div><h2>登录凭证</h2><p>邮箱和手机号均可用于登录，修改时必须完成验证码校验。</p></div>
      </div>
      <div class="contact-list">
        <div class="contact-row">
          <div class="contact-icon">邮</div>
          <div class="contact-info">
            <strong>绑定邮箱</strong>
            <span>{{ user?.email || '暂未绑定' }}</span>
          </div>
          <el-tag :type="user?.email ? 'success' : 'info'" effect="light">{{ user?.email ? '已绑定' : '未绑定' }}</el-tag>
          <el-button type="primary" plain @click="openContactDialog('email')">{{ user?.email ? '更换邮箱' : '绑定邮箱' }}</el-button>
        </div>
        <div class="contact-row">
          <div class="contact-icon phone">手</div>
          <div class="contact-info">
            <strong>绑定手机号</strong>
            <span>{{ user?.phone || '暂未绑定' }}</span>
          </div>
          <el-tag :type="user?.phone ? 'success' : 'info'" effect="light">{{ user?.phone ? '已绑定' : '未绑定' }}</el-tag>
          <el-button type="primary" plain :disabled="!user?.email" @click="openContactDialog('phone')">{{ user?.phone ? '更换手机号' : '绑定手机号' }}</el-button>
        </div>
      </div>
      <p v-if="!user?.email" class="security-tip">为保证账号安全，请先绑定邮箱；绑定邮箱后才能维护手机号。</p>
    </section>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="460px" destroy-on-close :close-on-click-modal="false">
      <p class="dialog-description">{{ verificationDescription }}</p>
      <el-form label-position="top">
        <el-form-item :label="form.type === 'email' ? '新邮箱' : '新手机号'">
          <el-input v-model.trim="form.value" :placeholder="form.type === 'email' ? '请输入新邮箱' : '请输入新手机号'" />
        </el-form-item>
        <el-form-item label="验证码">
          <div class="code-row">
            <el-input v-model.trim="form.code" maxlength="6" inputmode="numeric" placeholder="请输入 6 位验证码" />
            <el-button :loading="sendingCode" :disabled="countdown > 0" @click="sendCode">{{ countdown > 0 ? `${countdown} 秒后重发` : '获取验证码' }}</el-button>
          </div>
        </el-form-item>
      </el-form>
      <p v-if="verificationTarget" class="sent-tip">验证码已发送至 {{ verificationTarget }}</p>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="saveContact">确认修改</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.account-page { min-height: 420px; }
.account-panel { margin-bottom: 20px; padding: 24px; border: 1px solid #e4eaf2; border-radius: 14px; background: #fff; }
.identity-panel { display: flex; align-items: center; gap: 24px; }
.identity-panel h2, .section-heading h2 { margin: 0 0 6px; color: #172033; font-size: 20px; }
.identity-panel p, .section-heading p { margin: 0 0 10px; color: #7b8799; font-size: 13px; }
.avatar-editor { display: grid; flex: 0 0 112px; justify-items: center; gap: 8px; }
.avatar-editor small { color: #9aa4b2; font-size: 11px; text-align: center; line-height: 1.5; }
.avatar-input { display: none; }
.avatar-large { position: relative; display: grid; place-items: center; width: 76px; height: 76px; padding: 0; overflow: hidden; border: 0; border-radius: 22px; color: #fff; background: linear-gradient(145deg, #1677ff, #0ea5e9); font: inherit; font-size: 28px; font-weight: 800; cursor: pointer; }
.avatar-large img { width: 100%; height: 100%; object-fit: cover; }
.avatar-large em { position: absolute; right: 0; bottom: 0; left: 0; padding: 5px 0; color: #fff; background: rgb(15 23 42 / 66%); font-size: 11px; font-style: normal; font-weight: 500; opacity: 0; transition: opacity .16s ease; }
.avatar-large:hover em, .avatar-large:focus-visible em { opacity: 1; }
.identity-content { flex: 1; min-width: 0; }
.identity-title { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.name-readonly { display: flex; min-height: 34px; align-items: center; gap: 14px; margin-top: 12px; color: #526075; font-size: 14px; }
.name-editor { display: grid; grid-template-columns: minmax(200px, 420px) 76px 76px; gap: 10px; margin-top: 12px; }
.contact-list { margin-top: 18px; border-top: 1px solid #edf1f6; }
.contact-row { display: grid; grid-template-columns: 44px minmax(0, 1fr) auto 104px; align-items: center; gap: 14px; padding: 20px 0; border-bottom: 1px solid #edf1f6; }
.contact-icon { display: grid; place-items: center; width: 42px; height: 42px; border-radius: 12px; color: #1677ff; background: #edf5ff; font-weight: 800; }
.contact-icon.phone { color: #16a36a; background: #ebfaf3; }
.contact-info { display: grid; gap: 5px; }
.contact-info strong { color: #253047; font-size: 15px; }
.contact-info span, .dialog-description, .sent-tip, .security-tip { color: #7b8799; font-size: 13px; }
.security-tip { margin: 16px 0 0; padding: 12px 14px; border-radius: 8px; color: #9a6700; background: #fff8e8; }
.dialog-description { margin: -4px 0 18px; line-height: 1.7; }
.code-row { display: grid; grid-template-columns: minmax(0, 1fr) 128px; gap: 10px; width: 100%; }
.sent-tip { margin: 0; color: #16a36a; }
@media (max-width: 640px) { .identity-panel { align-items: flex-start; }.avatar-editor { flex-basis: 86px; }.name-editor { grid-template-columns: 1fr; }.contact-row { grid-template-columns: 42px minmax(0, 1fr) auto; }.contact-row .el-button { grid-column: 2 / 4; width: 100%; } }
</style>
