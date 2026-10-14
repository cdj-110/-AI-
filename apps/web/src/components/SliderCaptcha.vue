<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
import { RefreshRight, SuccessFilled } from '@element-plus/icons-vue';
import { apiRequest } from '../api/request';

type TrailPoint = { x: number; t: number };
type Challenge = {
  challengeId: string;
  imageWidth: number;
  pieceSize: number;
  pieceY: number;
  background: string;
  piece: string;
  expiresIn: number;
};

const emit = defineEmits<{ verified: [token: string] }>();
const shell = ref<HTMLElement>();
const challenge = ref<Challenge>();
const loading = ref(false);
const verifying = ref(false);
const verified = ref(false);
const successSeconds = ref(0);
const successPercentile = ref<number | null>(null);
const dragging = ref(false);
const sliderX = ref(0);
const message = ref('按住滑块，拖动完成拼图');
const startClientX = ref(0);
const startSliderX = ref(0);
const startedAt = ref(0);
const trail = ref<TrailPoint[]>([]);
let expiryTimer = 0;

const shellWidth = computed(() => shell.value?.clientWidth || 320);
const maxSliderX = computed(() => Math.max(1, shellWidth.value - 44));
const maxPieceX = computed(() => challenge.value ? challenge.value.imageWidth - challenge.value.pieceSize : 272);
const pieceLeft = computed(() => `${(sliderX.value / maxSliderX.value) * Math.max(1, shellWidth.value - (challenge.value?.pieceSize || 48))}px`);
const pieceTop = computed(() => `${((challenge.value?.pieceY || 0) / (challenge.value?.imageWidth || 320)) * shellWidth.value}px`);

async function load() {
  window.clearTimeout(expiryTimer);
  loading.value = true;
  verified.value = false;
  successSeconds.value = 0;
  successPercentile.value = null;
  dragging.value = false;
  sliderX.value = 0;
  trail.value = [];
  message.value = '按住滑块，拖动完成拼图';
  emit('verified', '');
  try {
    challenge.value = await apiRequest<Challenge>({ url: '/api/auth/captcha', method: 'GET' });
    expiryTimer = window.setTimeout(() => {
      if (!verified.value) {
        message.value = '验证已失效，请刷新';
        challenge.value = undefined;
      }
    }, challenge.value.expiresIn * 1000);
  } finally {
    loading.value = false;
  }
}

function offsetFromSlider(value = sliderX.value) {
  return Math.round((value / maxSliderX.value) * maxPieceX.value);
}

function begin(event: PointerEvent) {
  if (!challenge.value || loading.value || verifying.value || verified.value) return;
  dragging.value = true;
  startClientX.value = event.clientX;
  startSliderX.value = sliderX.value;
  startedAt.value = performance.now();
  trail.value = [{ x: offsetFromSlider(), t: 0 }];
  (event.currentTarget as HTMLElement).setPointerCapture?.(event.pointerId);
  event.preventDefault();
}

function move(event: PointerEvent) {
  if (!dragging.value) return;
  const next = Math.max(0, Math.min(maxSliderX.value, startSliderX.value + event.clientX - startClientX.value));
  sliderX.value = next;
  const elapsed = Math.min(15_000, Math.round(performance.now() - startedAt.value));
  const last = trail.value[trail.value.length - 1];
  const point = { x: offsetFromSlider(next), t: elapsed };
  if (!last || point.t - last.t >= 12) trail.value.push(point);
  if (trail.value.length > 159) trail.value.splice(1, 1);
}

async function end() {
  if (!dragging.value || !challenge.value) return;
  dragging.value = false;
  const elapsed = Math.min(15_000, Math.round(performance.now() - startedAt.value));
  const offsetX = offsetFromSlider();
  trail.value.push({ x: offsetX, t: elapsed });
  verifying.value = true;
  message.value = '正在验证…';
  try {
    const result = await apiRequest<{ captchaToken: string; expiresIn: number; durationMs: number; percentile: number | null }>({
      url: '/api/auth/captcha/verify',
      method: 'POST',
      data: { challengeId: challenge.value.challengeId, offsetX, trail: trail.value },
      silentError: true,
    });
    verified.value = true;
    successSeconds.value = Math.max(0.1, result.durationMs / 1000);
    successPercentile.value = result.percentile;
    message.value = result.percentile == null
      ? `验证通过，用时 ${successSeconds.value.toFixed(1)} 秒`
      : `用时 ${successSeconds.value.toFixed(1)} 秒，超过 ${result.percentile}% 的用户`;
    emit('verified', result.captchaToken);
  } catch {
    message.value = '未对准，请重新验证';
    window.setTimeout(load, 650);
  } finally {
    verifying.value = false;
  }
}

async function reset() {
  await nextTick();
  await load();
}

defineExpose({ reset });

onMounted(load);
onBeforeUnmount(() => window.clearTimeout(expiryTimer));
</script>

<template>
  <div class="captcha-box" :class="{ verified }">
    <div ref="shell" class="captcha-image">
      <div v-if="loading" class="captcha-loading">正在生成安全拼图…</div>
      <template v-else-if="challenge">
        <img class="captcha-background" :src="challenge.background" alt="滑块拼图背景" draggable="false" />
        <img
          v-if="!verified"
          class="captcha-piece"
          :src="challenge.piece"
          alt="拼图滑块"
          :style="{ left: pieceLeft, top: pieceTop, width: `${(challenge.pieceSize / challenge.imageWidth) * 100}%` }"
          draggable="false"
        />
        <div v-if="verified" class="captcha-success">
          <el-icon><SuccessFilled /></el-icon>
          <strong>安全验证已通过</strong>
          <span v-if="successPercentile == null">本次用时 {{ successSeconds.toFixed(1) }} 秒</span>
          <span v-else>用时 {{ successSeconds.toFixed(1) }} 秒，超过 {{ successPercentile }}% 的用户</span>
        </div>
      </template>
      <button type="button" class="captcha-refresh" title="换一张" aria-label="换一张安全拼图" :disabled="loading || verifying" @click="load">
        <el-icon><RefreshRight /></el-icon>
      </button>
    </div>
    <div class="captcha-track">
      <div class="captcha-track-fill" :style="{ width: `${sliderX + 22}px` }" />
      <span class="captcha-message">{{ message }}</span>
      <button
        type="button"
        class="captcha-handle"
        :class="{ dragging, verified }"
        :style="{ transform: `translateX(${sliderX}px)` }"
        :disabled="loading || verifying || verified || !challenge"
        aria-label="拖动滑块完成拼图验证"
        @pointerdown="begin"
        @pointermove="move"
        @pointerup="end"
        @pointercancel="end"
      >
        <el-icon v-if="verified"><SuccessFilled /></el-icon>
        <span v-else>››</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.captcha-box { width: 100%; user-select: none; }
.captcha-image { position: relative; width: 100%; aspect-ratio: 320 / 150; overflow: hidden; border: 1px solid #dce5f1; border-radius: 10px 10px 6px 6px; background: #edf3fb; }
.captcha-background { display: block; width: 100%; height: 100%; object-fit: cover; pointer-events: none; }
.captcha-piece { position: absolute; z-index: 2; aspect-ratio: 1; pointer-events: none; filter: drop-shadow(0 2px 4px rgba(15, 34, 62, .3)); }
.captcha-success { position: absolute; z-index: 4; inset: 0; display: grid; place-content: center; justify-items: center; gap: 5px; color: #087443; background: rgba(237, 253, 245, .92); text-align: center; backdrop-filter: blur(3px); }
.captcha-success .el-icon { margin-bottom: 2px; color: #16a05d; font-size: 28px; }
.captcha-success strong { font-size: 15px; }
.captcha-success span { color: #3b7d63; font-size: 12px; }
.captcha-loading { display: grid; height: 100%; place-items: center; color: #7890ac; font-size: 13px; }
.captcha-refresh { position: absolute; z-index: 3; top: 8px; right: 8px; display: grid; width: 30px; height: 30px; place-items: center; border: 1px solid rgba(255,255,255,.72); border-radius: 8px; color: #fff; background: rgba(10, 32, 65, .35); cursor: pointer; backdrop-filter: blur(4px); }
.captcha-refresh:disabled { cursor: not-allowed; opacity: .55; }
.captcha-track { position: relative; height: 44px; margin-top: 8px; overflow: hidden; border: 1px solid #dce5f1; border-radius: 8px; background: #f7f9fc; }
.captcha-track-fill { position: absolute; inset: 0 auto 0 0; border-right: 1px solid #8ec0ff; background: #e8f3ff; }
.captcha-message { position: absolute; inset: 0; display: grid; place-items: center; color: #8290a4; font-size: 13px; pointer-events: none; }
.captcha-handle { position: absolute; z-index: 2; inset: 0 auto 0 0; display: grid; width: 44px; height: 42px; place-items: center; border: 0; border-right: 1px solid #b8d7ff; color: #1677ff; background: #fff; font-size: 20px; font-weight: 700; cursor: grab; touch-action: none; transition: box-shadow .16s ease; }
.captcha-handle.dragging { cursor: grabbing; box-shadow: 0 0 0 3px rgba(22,119,255,.13); }
.captcha-handle.verified { border-color: #5bc58a; color: #16a05d; background: #ecfbf3; cursor: default; }
.captcha-box.verified .captcha-track-fill { background: #e5f8ee; border-color: #75d5a1; }
.captcha-box.verified .captcha-message { color: #118451; font-weight: 600; }
</style>
