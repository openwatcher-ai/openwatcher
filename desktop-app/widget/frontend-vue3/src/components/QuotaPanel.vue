<template>
  <section data-ui-id="quota-panel" class="quadrant quota-panel">
    <div class="panel-heading">
      <span class="eyebrow">额度</span>
    </div>
    <div class="quota-content">
      <div data-ui-id="quota-rings" class="quota-rings">
        <QuotaRing
          label="5h"
          :window-data="quota?.fiveHour"
          color="#ffad32"
          :status="quotaStatus"
        />
        <QuotaRing
          label="7d"
          :window-data="quota?.weekly"
          color="#a975ff"
          :status="quotaStatus"
        />
      </div>
      <dl class="quota-details">
        <div><dt>当前方案</dt><dd>{{ quota?.planType || '计划未知' }}</dd></div>
        <div><dt>重置时间（5h）</dt><dd>{{ resetLabel(quota?.fiveHour?.resetAt) }}</dd></div>
        <div><dt>重置时间（7d）</dt><dd>{{ resetLabel(quota?.weekly?.resetAt) }}</dd></div>
        <div><dt>额度状态</dt><dd>{{ quotaNote }}</dd></div>
      </dl>
    </div>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import QuotaRing from './QuotaRing.vue'

const props = defineProps({ quota: Object })
const cached = computed(() => props.quota?.fresh === false || props.quota?.status === 'stale')
const quotaStatus = computed(() => cached.value ? '已缓存' : '正常')
const quotaNote = computed(() => {
  if (!props.quota) return '数据不可用'
  if (props.quota.status === 'unavailable') return '暂不可用'
  return cached.value ? '数据已缓存' : '服务正常'
})

function resetLabel(resetAt) {
  if (!resetAt) return '—'
  const seconds = Math.max(0, resetAt - Date.now() / 1000)
  if (seconds < 3600) return `${Math.ceil(seconds / 60)} 分钟后`
  if (seconds < 24 * 3600) return `${Math.floor(seconds / 3600)} 小时后`
  return `${Math.floor(seconds / 86400)} 天后`
}
</script>
