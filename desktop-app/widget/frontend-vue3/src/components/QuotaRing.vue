<template>
  <div
    class="quota-ring"
    :class="{ muted: !windowData }"
    tabindex="0"
    :aria-label="ariaLabel"
  >
    <svg viewBox="0 0 120 120" aria-hidden="true">
      <circle class="time-track" cx="60" cy="60" r="55" />
      <circle
        class="time-value"
        :style="{ strokeDasharray: `${timePercent * 3.456} 345.6` }"
        cx="60"
        cy="60"
        r="55"
      />
      <circle class="ring-track" cx="60" cy="60" r="48" />
      <circle
        class="ring-value"
        :style="{ stroke: color, strokeDasharray: `${value * 3.02} 302` }"
        cx="60"
        cy="60"
        r="48"
      />
    </svg>
    <div class="ring-center">
      <strong>{{ windowData ? `${Math.round(value)}%` : '—' }}</strong>
      <span>{{ label }}</span>
    </div>
    <div class="ring-meta">
      <span>{{ windowData ? resetLabel : '暂无数据' }}</span>
      <small>{{ status }}</small>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'

const props = defineProps({
  label: String,
  windowData: Object,
  color: String,
  status: { type: String, default: '正常' },
})

const now = ref(Date.now() / 1000)
let timer
const value = computed(() => Math.max(0, Math.min(100, props.windowData?.remainingPercent ?? 0)))
const remainingSeconds = computed(() =>
  props.windowData?.resetAt ? Math.max(0, props.windowData.resetAt - now.value) : null,
)
const windowSeconds = computed(() => props.label === '5h' ? 5 * 3600 : 7 * 24 * 3600)
const timePercent = computed(() =>
  remainingSeconds.value === null ? 0 : Math.min(100, remainingSeconds.value / windowSeconds.value * 100),
)
const resetLabel = computed(() => {
  const seconds = remainingSeconds.value
  if (seconds === null) return '重置时间未知'
  if (seconds < 3600) return `${Math.ceil(seconds / 60)} 分钟后重置`
  if (seconds < 24 * 3600) return `${Math.floor(seconds / 3600)} 小时后重置`
  return `${Math.floor(seconds / 86400)} 天后重置`
})
const ariaLabel = computed(() =>
  props.windowData
    ? `${props.label} 剩余额度 ${Math.round(value.value)}%，${resetLabel.value}`
    : `${props.label} 额度暂无数据`,
)

onMounted(() => {
  timer = setInterval(() => {
    now.value = Date.now() / 1000
  }, 30000)
})
onUnmounted(() => clearInterval(timer))
</script>
