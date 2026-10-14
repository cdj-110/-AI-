<script setup lang="ts">
import { closeConfirm, useConfirmState } from '../confirm'
import ConfirmationDialog from './ConfirmationDialog.vue'

const state = useConfirmState()
</script>

<template>
  <ConfirmationDialog
    :open="state.open"
    :title="state.title"
    :message="state.message"
    :detail="state.detail"
    :confirm-text="state.confirmText"
    :cancel-text="state.cancelText"
    :danger="state.danger"
    :kind="state.kind"
    :fields="state.fields"
    @cancel="closeConfirm(false)"
    @confirm="closeConfirm(true)"
  >
    <template v-if="state.discardText" #actions><button type="button" class="danger" @click="closeConfirm('discard')">{{state.discardText}}</button></template>
  </ConfirmationDialog>
</template>
