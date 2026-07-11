<template>
  <section data-ui-id="today-panel" class="quadrant today-panel">
    <div class="panel-heading">
      <span class="eyebrow">今日 24 小时</span>
    </div>
    <div class="today-total">
      <Flame :size="18" />
      <strong>{{ today ? format(today.totalTokens) : '今日数据不可用' }}</strong>
    </div>
    <HourlyBarChart
      :buckets="buckets"
      :selected="selected"
      @select="$emit('select', $event)"
      @hover="$emit('hover', $event)"
    />
    <div class="axis" aria-hidden="true">
      <span v-for="label in ['00', '04', '08', '12', '16', '20', '24']" :key="label">{{ label }}</span>
    </div>
    <div data-ui-id="today-composition" class="composition-block">
      <UsageCompositionBar :parts="parts" :chr="chr" />
    </div>
    <div class="today-summary">
      <span>今日总量 <b>{{ format(today?.totalTokens) }}</b></span>
      <span>
        API 折算价值
        <b>{{ today?.valueLabel || '—' }}</b>
        <small v-if="today && !today.valueLabel">今日价值不可用</small>
      </span>
    </div>
    <Tooltip :text="tooltip?.kind === 'hour' ? tooltip.text : ''" />
  </section>
</template>

<script setup>
import { computed } from 'vue'
import { Flame } from '@lucide/vue'
import { composition, formatCompact } from '../state/widget.js'
import HourlyBarChart from './HourlyBarChart.vue'
import Tooltip from './Tooltip.vue'
import UsageCompositionBar from './UsageCompositionBar.vue'

const props = defineProps({
  today: Object,
  buckets: Array,
  selected: Object,
  tooltip: Object,
})
defineEmits(['select', 'hover'])

const parts = computed(() => composition(props.today))
const chr = computed(() => {
  if (!props.today?.inputTokens) return '—'
  return `${Math.round(props.today.cachedInputTokens / props.today.inputTokens * 100)}%`
})
const format = formatCompact
</script>
