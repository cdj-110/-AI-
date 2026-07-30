<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { closeConfirm, useConfirmState } from '../confirm'

const state = useConfirmState()
const confirmButton = ref<HTMLButtonElement>()

function cancel() {
  closeConfirm(false)
}

function confirm() {
  closeConfirm(true)
}

function onKeydown(event: KeyboardEvent) {
  if (!state.open || event.key !== 'Escape') return
  event.preventDefault()
  cancel()
}

watch(() => state.open, open => {
  if (open) nextTick(() => confirmButton.value?.focus())
})
onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <Teleport to="body">
    <div v-if="state.open" class="restart-modal global-confirm-modal" @click.self="cancel">
      <div class="dialog-panel" role="alertdialog" aria-modal="true" aria-labelledby="global-confirm-title" aria-describedby="global-confirm-message">
        <h3 id="global-confirm-title">{{ state.title }}</h3>
        <p id="global-confirm-message">{{ state.message }}</p>
        <p v-if="state.detail" class="global-confirm-detail">{{ state.detail }}</p>
        <div class="dialog-actions">
          <button type="button" @click="cancel">{{ state.cancelText }}</button>
          <button ref="confirmButton" type="button" :class="{ danger: state.danger, primary: !state.danger }" @click="confirm">{{ state.confirmText }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
