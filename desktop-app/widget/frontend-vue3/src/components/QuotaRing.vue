<template>
  <div
    class="quota-ring"
    :class="{ muted: !windowData, 'without-time': timePercent === null }"
    tabindex="0"
    :aria-label="ariaLabel"
  >
    <svg viewBox="0 0 120 120" aria-hidden="true">
      <defs>
        <linearGradient :id="gradientId" x1="0%" y1="100%" x2="100%" y2="0%">
          <stop offset="0%" stop-color="#ff4221" />
          <stop offset="34%" stop-color="#ffbc27" />
          <stop offset="68%" stop-color="#d7f60f" />
          <stop offset="100%" stop-color="#74f34a" />
        </linearGradient>
      </defs>
      <g class="ring-arcs" transform="rotate(115 60 60)">
        <circle class="quota-track" cx="60" cy="60" r="51" pathLength="100" />
        <circle
          class="quota-glow"
          :style="{ stroke: `url(#${gradientId})`, strokeDasharray: `${quotaArc} ${100 - quotaArc}` }"
          cx="60"
          cy="60"
          r="51"
          pathLength="100"
        />
        <circle
          class="quota-value"
          :style="{ stroke: `url(#${gradientId})`, strokeDasharray: `${quotaArc} ${100 - quotaArc}` }"
          cx="60"
          cy="60"
          r="51"
          pathLength="100"
        />
        <template v-if="timePercent !== null">
          <circle class="time-track" cx="60" cy="60" r="41" pathLength="100" />
          <circle
            class="time-glow"
            :style="{ strokeDasharray: `${timeArc} ${100 - timeArc}` }"
            cx="60"
            cy="60"
            r="41"
            pathLength="100"
          />
          <circle
            class="time-value"
            :style="{ strokeDasharray: `${timeArc} ${100 - timeArc}` }"
            cx="60"
            cy="60"
            r="41"
            pathLength="100"
          />
        </template>
      </g>
    </svg>
    <div class="ring-center">
      <strong>{{ windowData ? `${Math.round(value)}%` : '—' }}</strong>
      <span class="ring-countdown">{{ windowData ? compactResetLabel : '等待数据' }}</span>
      <span class="ring-orb-label">{{ label }}</span>
    </div>
    <div class="ring-meta">
      <strong>{{ label }}</strong>
    </div>
  </div>
</template>

<script setup>
import { computed, getCurrentInstance, onMounted, onUnmounted, ref } from 'vue'

const props = defineProps({
  label: String,
  windowData: Object,
})

const now = ref(Date.now() / 1000)
let timer
const gradientId = `quota-gradient-${getCurrentInstance()?.uid ?? 'ring'}`
const value = computed(() => Math.max(0, Math.min(100, props.windowData?.remainingPercent ?? 0)))
const remainingSeconds = computed(() =>
  props.windowData?.resetAt ? Math.max(0, props.windowData.resetAt - now.value) : null,
)
const windowSeconds = computed(() => props.label === '5h' ? 5 * 3600 : 7 * 24 * 3600)
const timePercent = computed(() =>
  remainingSeconds.value === null ? null : Math.min(100, remainingSeconds.value / windowSeconds.value * 100),
)
const quotaArc = computed(() => 86 * value.value / 100)
const timeArc = computed(() => 86 * (timePercent.value ?? 0) / 100)
const compactResetLabel = computed(() => {
  const seconds = remainingSeconds.value
  if (seconds === null) return '重置未知'
  if (seconds < 3600) return `${Math.ceil(seconds / 60)}m`
  if (seconds < 24 * 3600) {
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor(seconds % 3600 / 60)
    return minutes ? `${hours}h ${minutes}m` : `${hours}h`
  }
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor(seconds % 86400 / 3600)
  return hours ? `${days}d ${hours}h` : `${days}d`
})
const ariaLabel = computed(() =>
  props.windowData
    ? `${props.label} 剩余额度 ${Math.round(value.value)}%，剩余时间 ${compactResetLabel.value}`
    : `${props.label} 额度暂无数据`,
)

onMounted(() => {
  timer = setInterval(() => {
    now.value = Date.now() / 1000
  }, 30000)
})
onUnmounted(() => clearInterval(timer))
</script>
