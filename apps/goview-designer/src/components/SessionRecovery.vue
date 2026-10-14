<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { JSONStringify } from '@/utils'
import { acceptRecoveredSession, recoveryDraftKey, recoveryIdentity, recoveryPlatformOrigin, sessionRecovery, setRecoveryDraftSaver, showRecoveryLogin } from '@/utils/sessionRecovery'

const frame = ref<HTMLIFrameElement>()
const frameURL = ref('')
const frameHeight = ref(680)
let nonce = ''
let origin = ''
let accepting = false
const store = useChartEditStore()
const stopSaver = setRecoveryDraftSaver(() => {
  if (!window.location.hash.startsWith('#/chart/home/')) return false
  if (!sessionRecovery.draftReady) return !!sessionStorage.getItem(recoveryDraftKey())
  try {
    const title = store.getCurrentCanvasTitle
    const pages = store.getPageCallList.map((page: any) => page.title === title ? { title, ...store.getStorageInfo() } : page)
    sessionStorage.setItem(recoveryDraftKey(), JSONStringify({ projectId: sessionStorage.getItem('wk_Config_id'), title, pages, savedAt: Date.now() }))
    return true
  } catch { return false }
})
function exportDraft() {
  const title = store.getCurrentCanvasTitle
  const pages = store.getPageCallList.map((page: any) => page.title === title ? { title, ...store.getStorageInfo() } : page)
  const url = URL.createObjectURL(new Blob([JSONStringify({ title, pages })], { type: 'application/json' }))
  const link = document.createElement('a')
  link.href = url; link.download = '云组态草稿.json'; link.click()
  URL.revokeObjectURL(url)
}
watch(() => sessionRecovery.visible, visible => {
  if (!visible) { frameURL.value = ''; return }
  nonce = crypto.randomUUID()
  origin = recoveryPlatformOrigin()
  frameURL.value = `${origin}/session-login`
}, { immediate: true })
async function receive(event: MessageEvent) {
  if (!sessionRecovery.visible || event.origin !== origin || event.source !== frame.value?.contentWindow) return
  if (event.data?.type === 'wk-recovery-ready') {
    frame.value?.contentWindow?.postMessage({ type: 'wk-recovery-init', nonce, ...recoveryIdentity() }, origin)
    return
  }
  if (event.data?.type === 'wk-recovery-size' && event.data.nonce === nonce && typeof event.data.height === 'number' && Number.isFinite(event.data.height)) {
    frameHeight.value = Math.min(900, Math.max(260, Math.ceil(event.data.height)))
    return
  }
  if (event.data?.type !== 'wk-recovery-auth' || event.data.nonce !== nonce || typeof event.data.token !== 'string' || accepting) return
  accepting = true
  try { await acceptRecoveredSession(event.data.token) }
  catch (error) { sessionRecovery.error = (error as Error).message }
  finally { accepting = false }
}
window.addEventListener('message', receive)
onBeforeUnmount(() => { stopSaver(); window.removeEventListener('message', receive) })
</script>

<template>
  <div v-if="sessionRecovery.visible" class="session-recovery-mask" role="dialog" aria-modal="true" aria-label="重新登录">
    <section class="session-recovery-panel">
      <div class="session-recovery-status">登录已过期，{{ sessionRecovery.draftSaved ? '当前画布已暂存' : '当前编辑内容仍保留，请勿关闭页面' }}</div>
      <iframe ref="frame" :src="frameURL" :style="{ height: `${frameHeight}px` }" title="平台登录" class="session-recovery-login" />
      <p v-if="sessionRecovery.error" class="session-recovery-error">{{ sessionRecovery.error }}</p>
      <div class="session-recovery-actions"><button @click="exportDraft">导出草稿</button><button @click="sessionRecovery.visible = false">稍后登录</button></div>
    </section>
  </div>
  <button v-else-if="sessionRecovery.paused" class="session-recovery-resume" @click="showRecoveryLogin">登录已过期 · 重新登录</button>
</template>

<style scoped>
.session-recovery-mask { position: fixed; inset: 0; z-index: 100000; display: grid; place-items: center; background: #0009; padding: 16px; }
.session-recovery-panel { width: min(420px, 100%); max-height: calc(100vh - 32px); overflow: auto; border-radius: 18px; background: white; box-shadow: 0 24px 80px #0005; }
.session-recovery-status { padding: 14px 20px; background: #eff6ff; color: #42638a; font-size: 13px; }
.session-recovery-login { display: block; border: 0; width: 100%; height: 680px; background: white; }
.session-recovery-error { margin: 0; padding: 0 20px 12px; color: #d03050; }
.session-recovery-actions { display: flex; justify-content: space-between; padding: 12px 20px; border-top: 1px solid #edf1f7; }
.session-recovery-actions button { color: #2878dc; border: 0; background: none; cursor: pointer; }
.session-recovery-resume { position: fixed; top: 70px; right: 24px; z-index: 99999; background: #fff; color: #2878dc; border: 1px solid #2878dc; border-radius: 8px; padding: 12px; cursor: pointer; }
</style>
