<script setup lang="ts">
import OfficialHeader from '../components/OfficialHeader.vue';
import Changelog from './Changelog.vue';
</script>

<template>
  <div class="official-changelog-page"><OfficialHeader /><main><Changelog /></main><footer>© 2026 WEIKONG IoT Cloud · 产品更新持续记录</footer></div>
</template>

<style scoped>
.official-changelog-page { min-height: 100vh; background: linear-gradient(180deg, #fff 0, #f5f8fc 280px); }.official-changelog-page main { width: min(1180px, calc(100% - 40px)); margin: 0 auto; padding: 55px 0 90px; }.official-changelog-page footer { padding: 34px 20px; border-top: 1px solid #e5ebf2; color: #95a1b1; background: #fff; font-size: 11px; text-align: center; }
@media (max-width: 600px) { .official-changelog-page main { width: calc(100% - 24px); padding-top: 30px; } }
</style>
