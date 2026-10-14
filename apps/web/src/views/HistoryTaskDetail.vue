<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router';
import HistoryTaskEditor from '../components/HistoryTaskEditor.vue';
const route=useRoute(),router=useRouter();
</script>
<template><div><el-button style="margin-bottom:18px" @click="router.push('/data-center')">返回任务列表</el-button><HistoryTaskEditor :key="String(route.params.id)" :task-id="String(route.params.id)"/></div></template>
