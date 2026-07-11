<template>
  <div ref="container" class="weekly-wrap" :style="{ '--heat-cell-size': `${cellSize}px` }">
    <div class="week-axis" aria-hidden="true">
      <span />
      <span v-for="hour in 24" :key="hour">{{ (hour - 1) % 2 === 0 ? String(hour - 1).padStart(2, '0') : '' }}</span>
    </div>
    <div ref="body" class="weekly-body">
      <div class="day-labels" aria-hidden="true">
        <span v-for="({ day }, row) in rows" :key="day?.date || row">{{ day?.date?.slice(5) || '—' }}</span>
      </div>
      <div ref="grid" data-ui-id="weekly-grid" class="weekly-grid" role="grid">
        <template v-for="({ day, hours }, row) in rows" :key="day?.date || row">
          <button
            v-for="(value, hour) in hours"
            :key="`${row}-${hour}`"
            class="heat-cell"
            :class="[
              value === null ? 'missing' : level(value),
              { selected: selected?.key === `${day?.date}-${hour}` },
            ]"
            :disabled="!day || value === null"
            :aria-label="label(day, hour, value)"
            @mouseenter="day && value !== null && $emit('hover', item(day, hour, value))"
            @mouseleave="$emit('hover', null)"
            @click="day && value !== null && $emit('select', item(day, hour, value))"
          />
        </template>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { week168, weekTooltip } from '../state/widget.js'

const props = defineProps({ days: Array, peak: Number, selected: Object })
defineEmits(['select', 'hover'])

const container = ref(null)
const body = ref(null)
const grid = ref(null)
const cellSize = ref(10)
const rows = computed(() => week168(props.days))
let observer

const level = (value) => !value ? 'l0' : `l${Math.min(5, Math.ceil(value / Math.max(1, props.peak || 0) * 5))}`
const label = (day, hour, value) => weekTooltip(day, hour, value)
const item = (day, hour, value) => ({
  kind: 'week',
  key: `${day.date}-${hour}`,
  date: day.date,
  hour,
  text: weekTooltip(day, hour, value),
})

function measure() {
  if (!grid.value || !body.value) return
  const widthSize = (grid.value.clientWidth - 23 * 3) / 24
  const heightSize = (body.value.clientHeight - 6 * 3) / 7
  cellSize.value = Math.max(4, Math.floor(Math.min(widthSize, heightSize)))
}

onMounted(async () => {
  await nextTick()
  if (typeof ResizeObserver !== 'undefined') {
    observer = new ResizeObserver(measure)
    observer.observe(container.value)
  }
  measure()
})
onUnmounted(() => observer?.disconnect())
</script>
