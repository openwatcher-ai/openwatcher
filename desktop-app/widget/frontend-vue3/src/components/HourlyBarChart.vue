<template>
  <div data-ui-id="today-bars" class="hourly-chart" role="list">
    <button
      v-for="(bucket, index) in normalized"
      :key="bucket?.hourStart || index"
      class="hour-bar"
      :class="{ selected: selected?.key === bucket?.hourStart, missing: !bucket }"
      :style="{ '--bar-height': `${height(bucket?.totalTokens)}%` }"
      :disabled="!bucket"
      :aria-label="hourTooltip(bucket, index)"
      role="listitem"
      @mouseenter="bucket && $emit('hover', item(bucket, index))"
      @mouseleave="$emit('hover', null)"
      @click="bucket && $emit('select', item(bucket, index))"
    >
      <span />
    </button>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { hourTooltip, hours24 } from '../state/widget.js'

const props = defineProps({
  buckets: { type: Array, default: () => [] },
  selected: Object,
})
defineEmits(['select', 'hover'])

const normalized = computed(() => hours24(props.buckets))
const peak = computed(() => Math.max(1, ...normalized.value.map((bucket) => bucket?.totalTokens || 0)))
const height = (value) => value == null ? 3 : Math.max(3, value / peak.value * 100)
const item = (bucket, hour) => ({
  kind: 'hour',
  key: bucket.hourStart,
  hour,
  text: hourTooltip(bucket, hour),
})
</script>
