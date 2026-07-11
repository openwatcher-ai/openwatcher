<template>
  <div
    ref="grid"
    data-ui-id="calendar-grid"
    class="calendar-wrap"
    role="grid"
    :style="{
      '--calendar-cell-size': `${cellSize}px`,
      gridTemplateRows: `14px repeat(${rowCount}, ${cellSize}px)`,
    }"
  >
    <span v-for="name in names" :key="name" class="weekday">{{ name }}</span>
    <button
      v-for="(day, index) in cells"
      :key="index"
      class="calendar-cell"
      :class="[
        day ? level(day.totalTokens) : 'placeholder',
        { selected: Boolean(day) && selected?.key === day.date },
      ]"
      :disabled="!day"
      :aria-label="day ? dayTooltip(day) : '无数据占位'"
      @mouseenter="day && $emit('hover', item(day))"
      @mouseleave="$emit('hover', null)"
      @click="day && $emit('select', item(day))"
    >
      {{ day?.date?.slice(8) || '' }}
    </button>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { buildHeatScale, calendarCells, dayTooltip, heatScaleLevel } from '../state/widget.js'

const props = defineProps({ days: Array, selected: Object })
defineEmits(['select', 'hover'])

const names = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']
const grid = ref(null)
const cellSize = ref(20)
const cells = computed(() => calendarCells(props.days))
const rowCount = computed(() => Math.max(5, Math.ceil(cells.value.length / 7)))
const values = computed(() => (props.days || []).map((day) => day.totalTokens || 0))
const scale = computed(() => buildHeatScale(values.value))
let observer

function level(value) {
  return `l${heatScaleLevel(value, scale.value)}`
}

const item = (day) => ({ kind: 'day', key: day.date, text: dayTooltip(day) })

function measure() {
  if (!grid.value) return
  const widthSize = (grid.value.clientWidth - 6 * 4) / 7
  const heightSize = (grid.value.clientHeight - 14 - rowCount.value * 4) / rowCount.value
  cellSize.value = Math.max(8, Math.floor(Math.min(widthSize, heightSize, 38)))
}

onMounted(async () => {
  await nextTick()
  if (typeof ResizeObserver !== 'undefined') {
    observer = new ResizeObserver(measure)
    observer.observe(grid.value)
  }
  measure()
})
watch(rowCount, async () => {
  await nextTick()
  measure()
})
onUnmounted(() => observer?.disconnect())
</script>
