<template>
  <section data-ui-id="overview-panel" class="overview-panel">
    <header data-ui-id="overview-header" class="overview-header" style="--wails-draggable: drag">
      <h1 data-ui-id="overview-title">用量概览</h1>
      <div class="header-actions" style="--wails-draggable: no-drag">
        <span class="status-dot" :class="state.status">{{ statusLabel }}</span>
        <time class="updated">{{ updated }}</time>
        <span v-if="refreshFeedback" class="refresh-feedback" role="status">{{ refreshFeedback }}</span>
        <button
          class="icon-button"
          :disabled="busy"
          aria-label="刷新数据"
          title="刷新"
          @click="$emit('refresh')"
        >
          <RefreshCw :size="16" :class="{ spinning: busy }" />
        </button>
        <button
          v-if="state.status === 'offline' || state.status === 'invalid_credential'"
          class="open-main"
          aria-label="打开 OpenWatcher"
          @click="$emit('open-main')"
        >
          打开 OpenWatcher
        </button>
        <button
          class="icon-button"
          aria-label="收起用量概览"
          title="收起"
          @click="$emit('close')"
        >
          <X :size="18" />
        </button>
      </div>
    </header>
    <div class="quadrants"><slot /></div>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import { RefreshCw, X } from '@lucide/vue'

const props = defineProps({
  state: Object,
  statusLabel: String,
  busy: Boolean,
  refreshFeedback: String,
})
defineEmits(['refresh', 'close', 'open-main'])

const updated = computed(() => {
  if (!props.state?.observedAt) return '等待数据'
  try {
    return new Intl.DateTimeFormat('zh-CN', {
      hour: '2-digit',
      minute: '2-digit',
      timeZone: props.state.timezone || 'Asia/Shanghai',
    }).format(new Date(props.state.observedAt))
  } catch {
    return '时间未知'
  }
})
</script>
