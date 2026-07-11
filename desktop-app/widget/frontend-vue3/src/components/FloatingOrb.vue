<template>
  <button
    data-ui-id="floating-orb"
    class="floating-orb"
    :class="[anchorClass, status, { expanded, dragging, unavailable: slides.length === 0 }]"
    style="--wails-draggable: drag"
    aria-label="展开或收起用量概览"
    @click="click"
    @pointerdown="down"
    @pointermove="move"
    @pointerup="up"
    @pointercancel="up"
    @mouseenter="pause"
    @mouseleave="resume"
  >
    <Transition name="orb-crossfade">
      <span :key="active.key" class="orb-slide">
        <QuotaRing
          :label="active.label"
          :window-data="active.data"
          :color="active.color"
          status=""
        />
      </span>
    </Transition>
  </button>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import QuotaRing from './QuotaRing.vue'

const props = defineProps({
  quota: Object,
  status: String,
  expanded: Boolean,
  anchorCorner: { type: String, default: 'bottom-right' },
})
const emit = defineEmits(['toggle', 'drag-finished'])

const index = ref(0)
const paused = ref(false)
const dragging = ref(false)
let timer
let dragStart
let startedAt = 0
let remaining = 4000

const slides = computed(() => {
  if (props.quota?.status === 'unavailable') return []
  return [
    { key: '5h', label: '5h', data: props.quota?.fiveHour, color: '#ffad32' },
    { key: '7d', label: '7d', data: props.quota?.weekly, color: '#a975ff' },
  ].filter((slide) => slide.data)
})
const active = computed(() => slides.value[index.value] || {
  key: 'empty',
  label: '额度',
  data: null,
  color: '#586271',
})
const anchorClass = computed(() => `anchor-${props.anchorCorner || 'bottom-right'}`)

function schedule(delay = remaining) {
  clearTimeout(timer)
  if (paused.value || slides.value.length < 2) return
  remaining = Math.max(0, delay)
  startedAt = performance.now()
  timer = setTimeout(() => {
    index.value = (index.value + 1) % slides.value.length
    remaining = 4000
    schedule()
  }, remaining)
}

function pause() {
  if (paused.value) return
  paused.value = true
  if (timer) remaining = Math.max(0, remaining - (performance.now() - startedAt))
  clearTimeout(timer)
}

function resume() {
  if (!paused.value) return
  paused.value = false
  schedule()
}

function down(event) {
  dragStart = { x: event.screenX, y: event.screenY }
  dragging.value = false
}

function move(event) {
  if (dragStart && Math.hypot(event.screenX - dragStart.x, event.screenY - dragStart.y) > 4) {
    dragging.value = true
  }
}

function up() {
  if (dragging.value) emit('drag-finished')
  dragStart = null
  setTimeout(() => {
    dragging.value = false
  }, 0)
}

function click() {
  if (!dragging.value) emit('toggle')
}

watch(
  () => slides.value.map((slide) => slide.key).join(','),
  () => {
    if (index.value >= slides.value.length) index.value = 0
    remaining = 4000
    schedule()
  },
)

onMounted(() => schedule())
onUnmounted(() => clearTimeout(timer))
</script>
