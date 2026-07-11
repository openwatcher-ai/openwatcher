<template>
  <section data-ui-id="weekly-panel" class="quadrant weekly-panel">
    <div class="panel-heading">
      <span class="eyebrow">最近 7 天 × 24 小时</span>
    </div>
    <p v-if="!heatmap" class="local-empty">7 天热力数据不可用</p>
    <Heatmap7d
      v-else
      :days="heatmap.days"
      :selected="selected"
      @select="$emit('select', $event)"
      @hover="$emit('hover', $event)"
    />
    <span
      v-if="heatmap"
      class="legend"
      title="0 用量单独显示，非零用量按当前 7 天视图分为 5 个分位档"
    >低 <i /> 高</span>
    <Tooltip :text="tooltip?.kind === 'week' ? tooltip.text : ''" />
  </section>

  <section data-ui-id="trend-panel" class="quadrant trend-panel">
    <div class="panel-heading">
      <span class="eyebrow">最近 30 天</span>
    </div>
    <Calendar30d
      v-if="trend"
      :days="trend.days"
      :selected="selected"
      @select="$emit('select', $event)"
      @hover="$emit('hover', $event)"
    />
    <p v-else class="local-empty">30 天数据不可用</p>
    <div class="trend-summary">
      <span><small>累计</small><b>{{ format(trend?.totalTokens) }}</b></span>
      <span><small>峰值</small><b>{{ format(trend?.peakTokens) }}</b></span>
      <span><small>日均</small><b>{{ format(trend?.averageTokens) }}</b></span>
      <span class="trend-value">
        <b>{{ trend?.valueLabel || '—' }}</b><small>30 天价值</small>
        <small v-if="trend && !trend.valueLabel">价值不可用</small>
      </span>
    </div>
    <Tooltip :text="tooltip?.kind === 'day' ? tooltip.text : ''" />
  </section>
</template>

<script setup>
import { formatCompact } from '../state/widget.js'
import Calendar30d from './Calendar30d.vue'
import Heatmap7d from './Heatmap7d.vue'
import Tooltip from './Tooltip.vue'

defineProps({
  heatmap: Object,
  trend: Object,
  selected: Object,
  tooltip: Object,
})
defineEmits(['select', 'hover'])

const format = formatCompact
</script>
