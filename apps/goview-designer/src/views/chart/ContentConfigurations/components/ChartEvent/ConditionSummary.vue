<template>
  <div v-if="rules.length" class="condition-summary">
    <n-text depth="3" class="condition-relation">{{ group.leixingType || '满足全部条件' }} · {{ rules.length }} 条</n-text>
    <div v-for="(rule, index) in rules" :key="index" class="condition-rule">
      <span class="condition-number">{{ index + 1 }}.</span>
      <span>{{ conditionSummary(rule, components) }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { conditionSummary } from '@/utils/conditionSummary'
const props = defineProps<{ group?: any; components: any[] }>()
const rules = computed(() => Array.isArray(props.group?.tiaoList) ? props.group.tiaoList : [])
</script>

<style scoped>
.condition-summary { min-width: 0; font-size: 12px; }
.condition-relation { display: block; margin-bottom: 6px; }
.condition-rule { display: flex; gap: 6px; line-height: 1.6; overflow-wrap: anywhere; }
.condition-number { flex-shrink: 0; }
</style>
