<script setup lang="ts">
import { computed } from 'vue';
import { getAuthItem } from '../utils/auth-storage';
import logo from '../assets/tenant-platform-logo.png';

const signedIn = computed(() => Boolean(getAuthItem('accessToken')));
</script>

<template>
  <header class="official-header">
    <router-link class="official-brand" to="/" aria-label="微控物联网平台首页"><img :src="logo" alt="" /><strong>微控物联网平台</strong></router-link>
    <nav>
      <a href="/#capabilities">核心能力</a><a href="/#products">产品体系</a><router-link to="/changelog">更新日志</router-link>
    </nav>
    <div class="header-actions">
      <router-link v-if="!signedIn" class="text-link" to="/login">登录</router-link>
      <router-link class="console-link" :to="signedIn ? '/dashboard' : '/register'">{{ signedIn ? '进入控制台' : '免费开始' }}<span>→</span></router-link>
    </div>
  </header>
</template>

<style scoped>
.official-header { position: relative; z-index: 20; display: grid; grid-template-columns: 1fr auto 1fr; align-items: center; width: min(1180px, calc(100% - 40px)); height: 72px; margin: 0 auto; }
.official-brand { justify-self: start; display: flex; align-items: center; gap: 9px; color: #14243a; }.official-brand img { width: 34px; height: 34px; object-fit: cover; object-position: left center; }.official-brand strong { font-size: 15px; }
nav { display: flex; align-items: center; gap: 34px; }nav a { color: #536178; font-size: 14px; transition: color .2s; }nav a:hover { color: #1769e0; }
.header-actions { justify-self: end; display: flex; align-items: center; gap: 20px; }.text-link { color: #43536a; font-size: 14px; }.console-link { display: inline-flex; align-items: center; gap: 8px; padding: 10px 17px; border-radius: 9px; color: #fff; background: #1769e0; box-shadow: 0 7px 18px rgb(23 105 224 / 20%); font-size: 13px; font-weight: 600; }.console-link span { transition: transform .2s; }.console-link:hover span { transform: translateX(3px); }
@media (max-width: 760px) { .official-header { grid-template-columns: 1fr auto; width: min(100% - 24px, 1180px); }nav { display: none; }.official-brand strong { font-size: 14px; }.text-link { display: none; } }
</style>
