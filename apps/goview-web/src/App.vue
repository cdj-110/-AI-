<template>
  <el-config-provider :locale="appStore.elementLocale">
    <RouterView />
  </el-config-provider>
</template>

<script setup lang="ts">
import { useAppStore } from '@/stores/usePinia'

const appStore = useAppStore()
</script>

<style lang="scss">
*,
body,
html,
head,
header,
section,
footer,
select,
form,
div,
p,
span,
input,
img {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
}

:focus-visible {
    outline: none;
}

html, body {
    width: 100%;
    height: 100%;
    overflow: hidden;
}

#app {
    width: 100%;
    height: 100%;
    overflow: hidden;
}
</style>
