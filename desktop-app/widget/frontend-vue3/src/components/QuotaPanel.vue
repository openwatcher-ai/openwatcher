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
        />
        <QuotaRing
          label="7d"
          :window-data="quota?.weekly"
        />
      </div>
      <dl class="quota-details">
        <div>
          <span class="detail-icon plan"><Package :size="16" /></span>
          <dt>当前方案</dt><dd>{{ quota?.planType || '计划未知' }}</dd>
        </div>
        <div>
          <span class="detail-icon reset"><CalendarDays :size="16" /></span>
          <dt>重置时间（5h）</dt><dd>{{ resetLabel(quota?.fiveHour?.resetAt) }}</dd>
        </div>
        <div>
          <span class="detail-icon reset"><CalendarDays :size="16" /></span>
          <dt>重置时间（7d）</dt><dd>{{ resetLabel(quota?.weekly?.resetAt) }}</dd>
        </div>
        <div>
          <span class="detail-icon status"><Activity :size="16" /></span>
          <dt>额度状态</dt><dd>{{ quotaNote }}</dd>
        </div>
      </dl>
    </div>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import { Activity, CalendarDays, Package } from '@lucide/vue'
import QuotaRing from './QuotaRing.vue'

const props = defineProps({
  quota: Object,
  timezone: { type: String, default: '' },
})
const cached = computed(() => props.quota?.fresh === false || props.quota?.status === 'stale')
const quotaNote = computed(() => {
  if (!props.quota) return '数据不可用'
  if (props.quota.status === 'unavailable') return '暂不可用'
  return cached.value ? '数据已缓存' : '服务正常'
})

function resetLabel(resetAt) {
  if (!resetAt) return '—'
  try {
    const parts = new Intl.DateTimeFormat('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hourCycle: 'h23',
      timeZone: props.timezone || undefined,
    }).formatToParts(new Date(resetAt * 1000))
    const value = (type) => parts.find((part) => part.type === type)?.value || '--'
    return `${value('month')}-${value('day')} ${value('hour')}:${value('minute')}`
  } catch {
    return '时间未知'
  }
}
</script>
